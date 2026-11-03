# BUG 记录：PTP/IP 断开后 gRPC 连接被误关（HTTP/2 CONNECT_ERROR 10）

## 1. 摘要

在 PTP/IP 模式下断开相机后，Flutter（grpc-dart）侧会报：

```text
HTTP/2 error: Connection error:
Connection is being forcefully terminated. (errorCode: 10)
```

后端进程不会崩溃，`DisconnectCamera` 的日志也正常输出 `disconnected`。

根因**不在 Go 的断开逻辑**，而是：

> libgphoto2 2.5.34 的 `ptp2` camlib 在 PTP/IP 断开时，会对非富士相机执行 `close(jpgfd)`，而 `jpgfd` 恒为 0，导致进程的 `fd 0` 被关闭。下一次 gRPC 的 unix socket 被 `accept()` 分配到 `fd 0`，再下一次相机断开时又被 `close(0)` 误关，承载 RPC 的连接因此中断。

第一次断开通常正常，第二次开始必现。

---

## 2. 环境与版本

| 项 | 值 |
| --- | --- |
| 后端 | moment-go（cgo，macOS） |
| libgphoto2 | Homebrew `libgphoto2/2.5.34` |
| 相机驱动 | `.../libgphoto2/2.5.34/ptp2` |
| 相机连接 | PTP/IP，`ptpip:10.0.0.229:15740` |
| gRPC 传输 | unix socket `/tmp/moment_nkcon.sock` |
| 客户端 | Flutter + grpc-dart 5.x / http2 2.3.x |

---

## 3. 涉及的两条生命周期

这个问题本质上是**两条本应独立离开的生命周期，通过“进程 fd 表”耦合到了一起**。

### 3.1 gRPC 连接生命周期（长生命周期）

```text
进程启动
  → net.ListenConfig.Listen("unix", "/tmp/moment_nkcon.sock")
  → loggingListener.Accept()
      → accept() 返回当前进程“编号最小”的空闲 fd
  → 该 fd 在 Flutter 端整个 ClientChannel 存续期间一直占用
  → ClientChannel.shutdown() 或进程退出时才关闭
```

关键点：

- `accept()` 永远返回**当前最小的空闲 fd**；
- 正常情况下 fd 0/1/2 在进程启动时就已被占用（Go runtime 会为缺失的 std fd 打开 `/dev/null`），所以 gRPC 连接通常从 fd 3 往上分配；
- 但**只要有代码把 fd 0 关掉**，后续任何 `accept()`/`open()`/`socket()` 都可能拿到 0。

### 3.2 相机会话生命周期（短生命周期）

```text
Connect
  → gp_camera_new()
  → gp_camera_set_abilities()
  → gp_camera_set_port_info()
  → gp_camera_init()
      → ptp2 camera_init()
      → ptp_ptpip_connect()
          → socket() 创建 cmdfd / evtfd
  会话期间 gp_camera_* 操作复用这两个 socket

Disconnect（pkg/moment.Client.Disconnect）
  → 取消 sessionCtx
  → 等待在途租约（refs == 0）
  → GP2Camera.Close()
      → gp_camera_exit()
          → ptp2 camera_exit()
              → 厂商收尾（Nikon EndLiveView / change camera mode 等）
              → ptp_closesession()
              → ptp_free_params()
              → ptp_ptpip_disconnect()
                  → close(cmdfd)
                  → close(evtfd)
                  → close(jpgfd)      ← 问题所在
      → gp_camera_free()
  → 状态置为 DISCONNECTED
```

从设计上看，`Disconnect` 只会关闭相机自己的 PTP socket，**与 gRPC 的 unix socket 毫无关系**。唯一的交集就是进程级 fd 表。

---

## 4. 根因

### 4.1 libgphoto2 2.5.34 的实现

`camlibs/ptp2/ptpip.c`（libgphoto2 源码）中：

```c
int
ptp_ptpip_disconnect (PTPParams* params) {
    if (params->cmdfd != PTPSOCK_INVALID) {
        PTPSOCK_CLOSE (params->cmdfd);
        params->cmdfd = PTPSOCK_INVALID;
    }
    if (params->evtfd != PTPSOCK_INVALID) {
        PTPSOCK_CLOSE (params->evtfd);
        params->evtfd = PTPSOCK_INVALID;
    }
    if (params->jpgfd != PTPSOCK_INVALID) {
        PTPSOCK_CLOSE (params->jpgfd);
        params->jpgfd = PTPSOCK_INVALID;
    }
    ...
}
```

而 `PTPParams` 由 `calloc()` 分配，`jpgfd` 初始为 **0**；`jpgfd` 只会在 `ptp_fujiptpip_connect()`（富士相机）中赋值。因此对 Nikon/Sony/Canon 等非富士 PTP/IP 相机：

```text
jpgfd == 0 && 0 != PTPSOCK_INVALID(-1)
→ close(0)
```

即：**每次 PTP/IP 断开，都会关闭进程的 fd 0。**

### 4.2 上游记录

- PR #1286 / issue #1287：`ptp2: do not close descriptor 0 in ptp_ptpip_disconnect`
  - Linux/macOS：`close(0)` 静默关闭 stdin，并释放 fd 0 供复用；
  - iOS：`close(0)` 触发 `EXC_GUARD`，直接杀进程。
- libgphoto2 master 已修复：在 `ptp_ptpip_connect()` 入口初始化

  ```c
  params->cmdfd = PTPSOCK_INVALID;
  params->evtfd = PTPSOCK_INVALID;
  params->jpgfd = PTPSOCK_INVALID;
  ```

- 2.5.34 是该修复之前的版本，因此仍然存在。

### 4.3 另一个相关隐患

`ptp_ptpip_connect()` 的失败分支中，`close(cmdfd)` / `close(evtfd)` 之后没有把字段置回 `PTPSOCK_INVALID`。随后 Go 侧在 `gp_camera_init` 失败时调用 `gp_camera_free()`，会再次 close 这两个已经关闭的 fd 号。如果这两个号码在两次 close 之间被其他 goroutine 复用（新的 gRPC 连接、SQLite、日志 HTTP 连接等），也会被误关。

---

## 5. 实际触发时序（来自 test.log）

### 5.1 第一次流程：正常

```text
14:47:32.656  grpc connection accepted fd=8
14:47:32.699  connecting camera target ptpip:10.0.0.229:15740
14:47:32.878  connect:after_init          fd0_open=true
14:47:32.915  close:before_exit           fd0_open=true
14:47:33.427  close:after_exit            fd0_open=false
14:47:33.428  close:exit  closed=[0 13 14] opened=[15]
14:47:33.429  disconnected
```

此时：

- libgphoto2 关闭了 `0`（fd 0）、`13`、`14`（PTP socket）；
- gRPC 连接是 `fd 8`，**没有受到影响**；
- 客户端正常收到响应；
- 但 **fd 0 已经空出来了**。

### 5.2 第二次流程：报错

```text
14:49:41.256  grpc connection accepted fd=0      ← 复用了第一次释放的 fd 0
14:49:41.300  connecting camera target ptpip:10.0.0.229:15740
14:49:42.004  connect:after_init          fd0_open=true
14:49:42.051  close:before_exit           fd0_open=true
14:49:42.564  close:after_exit            fd0_open=false
14:49:42.565  close:exit  closed=[0 8 13]        ← fd 0 正是这次的 gRPC 连接
14:49:42.566  disconnected
```

服务端仍然完成了断开并打印了 `disconnected`，但此时承载该 RPC 的 fd 0 已经被 `close(0)` 关闭，响应无法写回，Flutter 端于是收到 HTTP/2 的 `CONNECT_ERROR(10)`。

### 5.3 时序图

```text
时间轴（进程 fd 表视角）

初始:
  fd 0 = stdin/dev-null   fd 8 = gRPC 连接

第一次 Disconnect:
  ptp_ptpip_disconnect():
      close(13)  PTP cmd socket
      close(14)  PTP event socket
      close(0)   ← jpgfd == 0
  结果: fd 0 空闲
  gRPC(fd 8) 不受影响 → 第一次看似正常

Flutter 重新连接:
  accept() 返回最小空闲 fd = 0
  结果: fd 0 = gRPC 连接

第二次 Disconnect:
  ptp_ptpip_disconnect():
      close(13)
      close(8)
      close(0)   ← 此时 fd 0 就是 gRPC 连接
  结果: Disconnect RPC 的传输层被关闭
  客户端: CONNECT_ERROR(10)
```

---

## 6. 为什么 USB 不受影响

`ptp2` 的 `camera_exit()` 中只有：

```c
if (camera->port->type == GP_PORT_PTPIP) {
    ptp_ptpip_disconnect (params);
}
```

USB 连接不会调用 `ptp_ptpip_disconnect()`，因此不会 `close(0)`。

---

## 7. 临时解决方案（Go 侧，已实现）

在不重新编译 libgphoto2 的前提下，通过 Go 侧防护切断“fd 0 被释放 → 被 gRPC 复用”的链路。

### 7.1 相机 teardown 的 fd 0 守护

新增 `adapter/gphoto2/fd_guard.go`：

```go
func guardFD0(fn func()) {
    fdGuardMu.Lock()
    defer fdGuardMu.Unlock()

    saved, _ := unix.Dup(0)                  // 保存原 fd 0
    devnull, _ := unix.Open(os.DevNull, ...)
    unix.Dup2(devnull, 0)                    // fd 0 换成临时 /dev/null
    ...
    defer unix.Dup2(saved, 0)                // 结束后恢复原 fd 0

    fn()                                     // gp_camera_exit / gp_camera_free
}
```

效果：

- libgphoto2 的 `close(0)` 只会关闭临时 `/dev/null`；
- 函数返回前 `dup2` 把原 fd 0 恢复回来；
- **fd 0 在两次断开之间始终被占用**，gRPC `accept()` 不会再拿到 0。

应用位置：

- `GP2Camera.Close()`：`gp_camera_exit` + `gp_camera_free` 整体包在 `guardFD0` 中；
- `GP2Client.Connect()` 的 `gp_camera_init` 失败清理：`gp_camera_free` 同样包一层。

### 7.2 gRPC accept 的第二道防线

`server/server.go` 新增 `keepConnOffStdFD` / `moveConnOffStdFD`：

- 如果 `accept()` 返回的连接落在 `fd 0/1/2`：
  - 立即 `dup` 到高位 fd，并通过 `net.FileConn` 重新包装；
  - 关闭原来的 std fd；
  - 用 `/dev/null` 重新占住 fd 0。
- 日志中的 `fd` 字段会显示迁移后的高位 fd。

即使极端情况下 fd 0 在 `close` 与恢复之间被 accept 抢到，gRPC 连接也会马上搬走，不会被后续的 PTP 断开误关。

### 7.3 诊断日志

保留并通过 `slog` 输出：

- `grpc connection accepted`：accept 出来的 fd；
- `gphoto2 fd state`：`connect:after_init` / `close:before_exit` / `close:after_exit` / `close:after_free` 的 fd 0 状态；
- `gphoto2 fd diff`：teardown 前后消失/新增的 fd；
- `WARN gphoto2: process fd 0 was closed during camera teardown (mitigated by fd guard)...`：确认防护已生效（libgphoto2 仍然在关 fd 0，但关的是占位符）。

### 7.4 回归测试

- `adapter/gphoto2/fd_guard_test.go`：模拟 `close(0)`，连续多次验证 fd 0 被恢复；
- `server/server_test.go`：验证连接可以从 std fd 迁移到高位 fd。

修复后预期日志：

```text
grpc connection accepted fd=8            ← 不再出现 fd=0
gphoto2 fd state stage=close:after_exit fd0_open=false   ← 关的是 /dev/null 占位符
WARN  gphoto2: process fd 0 was closed ... (mitigated by fd guard)
gphoto2 fd diff stage=close:exit closed=[0 13 14]        ← 不含 gRPC 的 fd
INFO  disconnected
```

客户端不再收到 `CONNECT_ERROR(10)`。

### 7.5 临时方案的局限

- 只能消除“fd 0 被复用”这条确定性路径；
- `ptp_ptpip_connect()` 失败分支的 stale fd 双重 close 仍可能误关其他 fd（概率低但存在）；
- 因此**临时方案不是最终修复**。

---

## 8. 永久修复建议

给 libgphoto2 的 `ptp2` camlib 打补丁（注意是 `camlibs/ptp2/ptpip.c`，不是 iolib 的 `ptpip.so`）：

```diff
@@ int ptp_ptpip_connect (PTPParams* params, const char *address) {
     char        *addr, *s, *p;
     int         port, eventport, tries;
     struct sockaddr_in  saddr;
     uint16_t    ret;

+    params->cmdfd = PTPSOCK_INVALID;
+    params->evtfd = PTPSOCK_INVALID;
+    params->jpgfd = PTPSOCK_INVALID;
+
     GP_LOG_D ("connecting to %s.", address);
```

并补充失败分支：

```diff
     PTPSOCK_CLOSE (params->cmdfd);
+    params->cmdfd = PTPSOCK_INVALID;
     PTPSOCK_CLOSE (params->evtfd);
+    params->evtfd = PTPSOCK_INVALID;
```

重建范围：

- **macOS/Linux**：构建带补丁的 libgphoto2，用 `CGO_LDFLAGS`/pkg-config 指向新库；
- **Android**：替换 `libgphoto2-android/lib` 与 `jniLibs/arm64-v8a` 下的 `ptp2.so`（版本 2.5.34.1）。

---

## 9. 验证方法

1. 重新编译并启动后端；
2. 连续做至少两轮 PTP/IP 连接 → 断开；
3. 过滤日志：

   ```sh
   grep -E "connection accepted|fd state|fd diff|fd 0 was closed|disconnecting|disconnected" <日志>
   ```

4. 判定标准：
   - `grpc connection accepted` 的 `fd` 始终 ≥ 3；
   - `gphoto2 fd diff` 的 `closed` 列表中不包含 gRPC 的 fd；
   - `close:after_exit` 仍可能显示 `fd0_open=false`（libgphoto2 仍在关 fd 0），但客户端不再报错；
   - 可选：断开后 `lsof -p <PID> -d 0` 应显示 fd 0 仍然存在。

---

## 10. 变更文件

| 文件 | 说明 |
| --- | --- |
| `adapter/gphoto2/fd_guard.go` | 新增：`guardFD0` fd 0 守护 |
| `adapter/gphoto2/fd_diag.go` | 新增：fd 状态 / fd 集合差异诊断日志 |
| `adapter/gphoto2/fd_guard_test.go` | 新增：fd 0 恢复回归测试 |
| `adapter/gphoto2/client.go` | `Close` / Connect 失败清理接入 `guardFD0`，补充诊断日志 |
| `server/server.go` | accept 后把 std fd 上的连接迁移到高位 fd，并保留 `fd` 日志 |
| `server/server_test.go` | 新增：连接 fd 迁移测试 |
| `pkg/moment/client.go` | 断开前输出 `disconnecting camera` 日志 |
| `go.mod` | `golang.org/x/sys` 改为直接依赖 |

---

## 11. 后续可选改进

- `GP2Camera.Close` 中 `gp_camera_exit` 返回错误时只记录 warning，不再让 `DisconnectCamera` 返回业务错误（相机掉线时 `ptp_closesession` 可能超时）；
- 给 Disconnect 增加有界等待，把 `gp_camera_exit`/`gp_camera_free` 放到后台完成；
- `Client.Close` 串行化，避免 `CONNECTING`/`DISCONNECTING` 状态下提前释放 `GPContext`。

---

## 12. 补充：stdout 的 `connect cmd: Operation timed out` 与 Ctrl+C 无法退出

### 12.1 这行输出来自哪里

`connect cmd: Operation timed out` **不是 slog 打的**，而是 libgphoto2 的 `ptp2` camlib：

```c
// camlibs/ptp2/ptpip.c
if (ptpip_connect_with_timeout(...) == -1) {
    ptpip_perror("connect cmd");
    ...
}

void ptpip_perror (const char *what) {
    perror(what);   // 非 Windows：直接写 stderr
}
```

它出现在一次 PTP/IP 连接尝试中（例如客户端启动后自动连接相机、但相机不可达/未响应），PTP/IP 的 connect 超时为 `PTPIP_DEFAULT_TIMEOUT_S=2` + `PTPIP_DEFAULT_TIMEOUT_MS=500`，即 2.5s。

Go 侧没有注册 libgphoto2 的日志回调，因此这类 `perror` 输出无法进入 slog/otel 管线。它不代表程序崩溃，只是一次连接失败。

### 12.2 为什么 Ctrl+C 退不出去

三个因素叠加：

1. **`GracefulStop` 会等待所有在途 RPC**
   - `download.v1.DownloadService/MonitorTask` 是 **server streaming** 长连接；
   - 客户端不主动关闭时，`stream.Context()` 不会结束；
   - gRPC 的 `GracefulStop` 不会取消在途流的 context，因此会一直等；
   - 同理，如果 RPC 阻塞在 cgo（`gp_camera_init`、`gp_camera_exit`、大文件下载）里，也会一直等。

2. **首次 SIGINT 之后的 Ctrl+C 被吞掉**
   - `signal.NotifyContext` 在收到信号后只调用 `cancel()`，不会 `signal.Stop`；
   - 后续 SIGINT 仍被送进它的 channel，但处理 goroutine 已经退出，于是不会触发默认的进程终止。

3. **`Client.Close` 的 5s ctx 实际无效**
   - `Disconnect` 在 `c.ops.Lock()`、`c.idle.Wait()` 等位置不检查 ctx；
   - 因此即使传了 timeout，仍可能无限等待卡在 cgo 里的操作。

### 12.3 已实施的处理

| 位置 | 改动 |
| --- | --- |
| `server/server.go` | `shutdownGraceful`：先 `GracefulStop`，5s 超时后 `Stop()`；输出 `grpc graceful stop timed out, forcing stop` |
| `service/download/download.go` | `MonitorTask` 额外监听服务根 ctx，SIGINT 时主动结束长流 |
| `pkg/moment/client.go` | `Client.Close` 加硬超时；超时后跳过 `closeFn`，避免 cgo 仍在使用 `GPContext` 时释放它 |
| `app/main.go` | 显式信号处理：第一次 SIGINT 优雅关闭，**第二次 SIGINT 直接 `os.Exit(1)`** |
| `box/box.go` | `Box.Close` 各阶段打 debug 日志，便于定位卡在哪一步 |

### 12.4 验证

正常退出时日志中应出现：

```text
INFO  shutdown signal received signal=interrupt
DEBUG grpc server shutting down
DEBUG box: closing
DEBUG client close: disconnecting
DEBUG disconnecting camera port_path=ptpip:...
DEBUG gphoto2 fd state stage=close:before_exit ...
DEBUG gphoto2 fd state stage=close:after_exit ...
DEBUG client close: disconnect completed
DEBUG box: camera client closed
DEBUG box: closed
```

如果某个在途 RPC/流无法结束，会在 5s 后出现：

```text
WARN grpc graceful stop timed out, forcing stop timeout=5s
```

此时进程仍会继续退出。若仍然卡住，第二次 Ctrl+C 会打印 `second shutdown signal received, forcing exit` 并立即结束进程。

> 注意：`connect cmd: Operation timed out` 属于 libgphoto2 直接写 stderr 的行为，Go 侧无法通过 slog 接管；如需消除，只能重编 libgphoto2 或重定向进程 stderr（不推荐）。

