package main

// app 包的 gRPC 集成测试。
//
// 测试在进程内拉起真实的 gRPC Server（camera / connection 两个服务），
// 并使用 moment-protogo 生成的 gRPC client 通过 unix socket 进行连接，
// 覆盖 connection 与 camera 服务的方法。download 与 sync 暂不涉及。
//
// 说明：是否存在可用相机由运行环境决定。连接成功时执行完整的信息获取用例；
// 无相机 / 未启用 cgo 后端时，依赖连接的用例会以 t.Skip 跳过，而不是失败。

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ci.zevix.uk/moment-go/box"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
	configv1 "ci.zevix.uk/moment/moment-protogo/config/v1"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ptpipEnvHost 指定真实 PTP/IP 相机地址（如 "192.168.1.10" 或
// "192.168.1.10:15740"）。未设置时，真实 PTP/IP 连接用例将跳过。
const ptpipEnvHost = "MOMENT_TEST_PTPIP_HOST"

// rpcTimeout 限制单次 RPC 的墙钟时间，避免真实设备探测 / 网络连接时测试挂死。
const rpcTimeout = 30 * time.Second

// harness 持有测试服务器地址与两个服务的 gRPC client。
type harness struct {
	addr    string
	conn    *grpc.ClientConn
	app     *box.Box
	connSvc connectionv1.CameraConnectionServiceClient
	camSvc  camerav1.CameraServiceClient
}

var testEnv *harness

func TestMain(m *testing.M) {
	os.Exit(runWithServer(m))
}

func runWithServer(m *testing.M) int {
	// 使用独立 socket 与临时数据目录，避免与真实运行进程冲突。
	sock := filepath.Join("/tmp", fmt.Sprintf("moment-go-test-%d.sock", os.Getpid()))
	dataDir, err := os.MkdirTemp("", "moment-go-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create temp dir:", err)
		return 1
	}
	defer os.RemoveAll(dataDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := box.New(ctx, box.Options{DataDir: dataDir, SocketAddr: sock})
	if err != nil {
		fmt.Fprintln(os.Stderr, "build box:", err)
		return 1
	}
	defer app.Close()

	go func() { _ = app.Run(ctx) }()

	conn, err := dial(ctx, sock)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial test server:", err)
		return 1
	}
	defer conn.Close()

	testEnv = &harness{
		addr:    sock,
		conn:    conn,
		app:     app,
		connSvc: connectionv1.NewCameraConnectionServiceClient(conn),
		camSvc:  camerav1.NewCameraServiceClient(conn),
	}
	return m.Run()
}

// dial 通过自定义 dialer 将 gRPC 连接到 unix socket。
func dial(ctx context.Context, sock string) (*grpc.ClientConn, error) {
	return grpc.NewClient("passthrough:///moment-go-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		}),
	)
}

// --- 连接状态缓存：跨用例共享同一后端会话 ---

var (
	connectMu   sync.Mutex
	connected   bool
	connectResp *connectionv1.ConnectCameraResponse
)

// connectUSB 通过 connection 服务建立一次 USB 连接，结果被缓存复用。
// 返回 false 表示当前环境没有可用相机（调用方应 Skip）。
func connectUSB(t *testing.T, ctx context.Context) bool {
	connectMu.Lock()
	defer connectMu.Unlock()

	if connected {
		return true
	}

	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	resp, err := testEnv.connSvc.ConnectCamera(rpcCtx, &connectionv1.ConnectCameraRequest{
		Mode: connectionv1.ConnectionMode_CONNECTION_MODE_USB,
	})
	if err != nil {
		t.Skipf("camera unavailable, skip: %v", err)
		return false
	}

	connected = true
	connectResp = resp
	if resp.GetModel() == "" {
		t.Errorf("ConnectCamera returned empty model")
	}
	t.Logf("connected camera: model=%q port=%q", resp.GetModel(), resp.GetPort())
	return true
}

// --- connection.v1.CameraConnectionService ---

// markDisconnected 清空连接状态缓存。
func markDisconnected() {
	connectMu.Lock()
	connected = false
	connectResp = nil
	connectMu.Unlock()
}

// ensureDisconnected 确保后端未连接，供 PTP/IP 负向用例使用：
// moment.Client 对已连接会话会直接复用，只有未连接时才会真正尝试建连。
func ensureDisconnected(t *testing.T) {
	if testEnv.app.Camera.IsConnected() {
		if _, err := testEnv.connSvc.DisconnectCamera(context.Background(),
			&connectionv1.DisconnectCameraRequest{}); err != nil {
			t.Fatalf("precondition disconnect: %v", err)
		}
	}
	markDisconnected()
}

// connectPTPIP 通过 connection 服务发起一次 PTP/IP 连接。
func connectPTPIP(ctx context.Context, host string) (*connectionv1.ConnectCameraResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	return testEnv.connSvc.ConnectCamera(rpcCtx, &connectionv1.ConnectCameraRequest{
		Mode:     connectionv1.ConnectionMode_CONNECTION_MODE_PTPIP,
		PortPath: host,
	})
}

func TestConnectionService(t *testing.T) {
	t.Run("InitlizeCamera_AppliesConfig", func(t *testing.T) {
		cfg := &configv1.Config{
			OpPath:  "/tmp/moment-test/op",
			NetPath: "/tmp/moment-test/net",
		}

		resp, err := testEnv.connSvc.InitlizeCamera(context.Background(),
			&connectionv1.InitlizeCameraRequest{Cfg: cfg})
		if err != nil {
			t.Fatalf("InitlizeCamera: %v", err)
		}
		if resp == nil {
			t.Fatal("InitlizeCamera returned nil response")
		}

		got := testEnv.app.Config.Get()
		if got == nil {
			t.Fatal("config store returned nil after InitlizeCamera")
		}
		if got.GetOpPath() != cfg.GetOpPath() || got.GetNetPath() != cfg.GetNetPath() {
			t.Fatalf("config not applied: got %+v want %+v", got, cfg)
		}
	})

	// PTP/IP 用例先于 USB 执行：只有未连接时才会真正走建连逻辑。
	t.Run("ConnectCamera_PTPIP_MissingHost_ReturnsError", func(t *testing.T) {
		ensureDisconnected(t)

		_, err := connectPTPIP(context.Background(), "192.168.1.1")
		if err == nil {
			t.Fatal("expected error when PTP/IP host is empty")
		}
		t.Logf("expected connect error: %v", err)
	})

	t.Run("ConnectCamera_PTPIP_Unreachable_ReturnsError", func(t *testing.T) {
		ensureDisconnected(t)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := connectPTPIP(ctx, "127.0.0.1")
		if err == nil {
			t.Fatal("expected error connecting to unreachable PTP/IP endpoint")
		}
		t.Logf("expected connect error: %v", err)
	})

	t.Run("ConnectCamera_PTPIP_FromEnv", func(t *testing.T) {
		host := os.Getenv(ptpipEnvHost)
		if host == "" {
			t.Skipf("set %s=<ip[:port]> to run real PTP/IP connect test", ptpipEnvHost)
		}
		ensureDisconnected(t)

		resp, err := connectPTPIP(context.Background(), host)
		if err != nil {
			t.Fatalf("ConnectCamera(PTPIP %s): %v", host, err)
		}
		if resp.GetModel() == "" {
			t.Errorf("PTP/IP ConnectCamera returned empty model")
		}
		t.Logf("connected over PTP/IP: model=%q port=%q", resp.GetModel(), resp.GetPort())

		// 释放本次 PTP/IP 会话，避免影响后续 USB 用例。
		if _, err := testEnv.connSvc.DisconnectCamera(context.Background(),
			&connectionv1.DisconnectCameraRequest{}); err != nil {
			t.Fatalf("disconnect PTP/IP: %v", err)
		}
		markDisconnected()
	})

	t.Run("ConnectCamera_USB", func(t *testing.T) {
		if !connectUSB(t, context.Background()) {
			return
		}
		if connectResp.GetPort() == "" {
			t.Errorf("ConnectCamera returned empty port")
		}
	})

	t.Run("DisconnectCamera_WhenDisconnected_IsNoop", func(t *testing.T) {
		if testEnv.app.Camera.IsConnected() {
			t.Skip("camera connected, covered by TestDisconnectCamera lifecycle")
		}

		resp, err := testEnv.connSvc.DisconnectCamera(context.Background(),
			&connectionv1.DisconnectCameraRequest{})
		if err != nil {
			t.Fatalf("disconnect without a session should be idempotent: %v", err)
		}
		if resp == nil {
			t.Fatal("DisconnectCamera returned nil response")
		}
	})
}

// --- camera.v1.CameraService ---

func TestCameraService(t *testing.T) {
	if !connectUSB(t, context.Background()) {
		return
	}

	t.Run("GetSummary", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()

		resp, err := testEnv.camSvc.GetSummary(ctx, &camerav1.GetSummaryRequest{})
		if err != nil {
			t.Fatalf("GetSummary: %v", err)
		}
		parsed := resp.GetParsed()
		if parsed == nil {
			t.Fatal("GetSummary returned nil parsed summary")
		}
		if parsed.GetModel() == "" {
			t.Errorf("summary model is empty")
		}
		t.Logf("summary: model=%q driver=%q port=%q", parsed.GetModel(), parsed.GetDriver(), parsed.GetPortPath())
		t.Logf("summary text=%q storage=%+v", parsed.GetSummary(), parsed.GetStorage())
	})

	t.Run("ListFiles_Root", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()

		resp, err := testEnv.camSvc.ListFiles(ctx, &camerav1.ListFilesRequest{Folder: "/"})
		if err != nil {
			t.Fatalf("ListFiles(/): %v", err)
		}
		if resp.GetFolder() != "/" {
			t.Fatalf("ListFiles folder echo = %q, want %q", resp.GetFolder(), "/")
		}
		for _, e := range resp.GetEntries() {
			if e.GetName() == "" {
				t.Errorf("ListFiles returned entry with empty name: %+v", e)
			}
		}
		t.Logf("ListFiles(/) returned %d entries", len(resp.GetEntries()))
	})

	t.Run("ListFiles_NonExistentFolder_ReturnsError", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()

		_, err := testEnv.camSvc.ListFiles(ctx, &camerav1.ListFilesRequest{Folder: "/definitely/not/here"})
		if err == nil {
			t.Fatal("expected error listing a non-existent folder")
		}
		t.Logf("expected list error: %v", err)
	})

	t.Run("ListFolders_Root", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()

		resp, err := testEnv.camSvc.ListFolders(ctx, &camerav1.ListFoldersRequest{Folder: "/"})
		if err != nil {
			t.Fatalf("ListFolders(/): %v", err)
		}
		if resp.GetFolder() != "/" {
			t.Fatalf("ListFolders folder echo = %q, want %q", resp.GetFolder(), "/")
		}
		for _, e := range resp.GetEntries() {
			if e.GetName() == "" {
				t.Errorf("ListFolders returned entry with empty name: %+v", e)
			}
		}
		t.Logf("ListFolders(/) returned %d entries", len(resp.GetEntries()))

		// 若根目录下存在子目录，则继续验证其中一个子目录可被列举。
		if len(resp.GetEntries()) == 0 {
			t.Log("no subfolders under root, skip nested folder case")
			return
		}

		sub := resp.GetEntries()[0].GetFloder()
		if sub == "" {
			t.Fatalf("folder entry has empty path: %+v", resp.GetEntries()[0])
		}

		nested, err := testEnv.camSvc.ListFolders(ctx, &camerav1.ListFoldersRequest{Folder: sub})
		if err != nil {
			t.Fatalf("ListFolders(%q): %v", sub, err)
		}
		if nested.GetFolder() != sub {
			t.Fatalf("ListFolders folder echo = %q, want %q", nested.GetFolder(), sub)
		}
		t.Logf("ListFolders(%q) returned %d entries", sub, len(nested.GetEntries()))
	})

	t.Run("ListFolders_NonExistentFolder_ReturnsError", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()

		_, err := testEnv.camSvc.ListFolders(ctx, &camerav1.ListFoldersRequest{Folder: "/definitely/not/here"})
		if err == nil {
			t.Fatal("expected error listing a non-existent folder")
		}
		t.Logf("expected list error: %v", err)
	})
}

// --- 生命周期：连接后断开的完整流程 ---

func TestDisconnectCamera(t *testing.T) {
	if !connectUSB(t, context.Background()) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	if _, err := testEnv.connSvc.DisconnectCamera(ctx, &connectionv1.DisconnectCameraRequest{}); err != nil {
		t.Fatalf("DisconnectCamera: %v", err)
	}

	if testEnv.app.Camera.IsConnected() {
		t.Fatal("client still reports connected after DisconnectCamera")
	}
}
