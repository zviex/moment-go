//go:build cgo

package gphoto2

import (
	"log/slog"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// fdGuardMu 串行化 fd 0 的守护与恢复，避免多个相机会话同时 teardown 时
// 互相踩踏 fd 0。
var fdGuardMu sync.Mutex

// guardFD0 在 fn 执行期间把 fd 0 换成一个可丢弃的 /dev/null，并在结束后
// 恢复调用前的 fd 0。
//
// 背景：libgphoto2 2.5.34 的 ptp2 camlib 在 PTP/IP 断开时调用
// ptp_ptpip_disconnect()，其中会 close(jpgfd)。非富士 PTP/IP 相机上
// jpgfd 恒为 0（PTPParams 由 calloc 初始化），于是进程的 fd 0 被关闭。
// fd 0 一旦空出来，后续 gRPC 的 unix socket 可能被 accept 到 fd 0；
// 下一次相机断开时会再次 close(0)，把 gRPC 连接直接打断，客户端表现为
// HTTP/2 CONNECT_ERROR(10)。
//
// 这里在 teardown 前保存原 fd 0，并让 libgphoto2 关闭一个临时的
// /dev/null；结束后无论它是否真的关闭了 fd 0，都用 dup2 把原 fd 0 恢复
// 回来，保证 fd 0 始终被占用，不会被 accept 复用。
//
// 注意：如果 libgphoto2 在 close(0) 与恢复之间被其他 goroutine 抢占了
// fd 0，仍有极小概率误伤；server 端的 accept 包装会把落到 std fd 上的
// 连接迁移到高位 fd，作为第二道防线。彻底修复仍需给 libgphoto2 打补丁
// （ptp_ptpip_connect 中初始化 jpgfd = PTPSOCK_INVALID）。
func guardFD0(fn func()) {
	fdGuardMu.Lock()
	defer fdGuardMu.Unlock()

	saved, err := unix.Dup(0)
	if err != nil {
		slog.Warn("gphoto2: dup fd 0 failed, running teardown without fd guard", "err", err)
		fn()
		return
	}
	defer unix.Close(saved)

	devnull, err := unix.Open(os.DevNull, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		slog.Warn("gphoto2: open /dev/null failed, running teardown without fd guard", "err", err)
		fn()
		return
	}
	if err := unix.Dup2(devnull, 0); err != nil {
		unix.Close(devnull)
		slog.Warn("gphoto2: dup2 /dev/null -> fd 0 failed, running teardown without fd guard", "err", err)
		fn()
		return
	}
	if devnull != 0 {
		unix.Close(devnull)
	}

	// 无论 fn 是否 panic，都要恢复 fd 0。
	defer func() {
		if err := unix.Dup2(saved, 0); err != nil {
			slog.Warn("gphoto2: restore fd 0 failed", "err", err)
		}
	}()

	fn()
}
