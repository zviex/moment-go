package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"syscall"
	"time"

	"ci.zevix.uk/moment-go/server/mnet"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

// Server 是 gRPC 服务器，监听地址由注入的 mnet.Net 决定。
type Server struct {
	*grpc.Server
	net mnet.Net
}

// NewServer 创建服务器。network 为 nil 时退化为非监听模式（仅用于测试装配）。
func NewServer(network mnet.Net) *Server {
	return &Server{
		Server: grpc.NewServer(),
		net:    network,
	}
}

// RegisterService 注册生成的 gRPC 服务。
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl any) {
	s.Server.RegisterService(desc, impl)
}

func (s *Server) Run(ctx context.Context) error {
	if s.net == nil {
		return errors.New("server: no network configured")
	}

	netName := s.net.Network()
	addr := s.net.Addr()

	slog.Debug("grpc server listening", "network", netName, "addr", addr)

	nlc := net.ListenConfig{}

	nl, err := nlc.Listen(ctx, netName, addr)
	if err != nil {
		slog.Error("grpc server listen failed", "network", netName, "addr", addr, "err", err)
		return err
	}

	defer nl.Close()

	slog.Debug("grpc server started", "network", netName, "addr", addr)

	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(loggingListener{Listener: nl}) }()

	select {
	case err := <-errCh:
		slog.Debug("grpc server stopped", "addr", addr, "err", err)
		return err
	case <-ctx.Done():
		slog.Debug("grpc server shutting down", "addr", addr)
		shutdownGraceful(s.Server, addr)
		<-errCh
		slog.Debug("grpc server stopped", "addr", addr)
		return nil
	}
}

// shutdownTimeout 限制优雅关闭的最长等待时间。超过该时间仍有在途 RPC
// （例如阻塞在 cgo 的 gp_camera_init / gp_camera_exit，或长生命周期流），
// 就强制 Stop，避免 SIGINT 后进程一直卡住。
var shutdownTimeout = 5 * time.Second

// shutdownGraceful 先尝试 GracefulStop，超时后强制 Stop。
func shutdownGraceful(s *grpc.Server, addr string) {
	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Debug("grpc graceful stop completed", "addr", addr)
	case <-time.After(shutdownTimeout):
		slog.Warn("grpc graceful stop timed out, forcing stop",
			"addr", addr, "timeout", shutdownTimeout)
		s.Stop()
		<-done
	}
}

// loggingListener 在每次接受连接时打点，便于观察连接建立状态。
type loggingListener struct {
	net.Listener
}

func (l loggingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		if !errors.Is(err, net.ErrClosed) {
			slog.Debug("grpc accept failed", "err", err)
		}
		return nil, err
	}

	// libgphoto2 2.5.34 的 PTP/IP teardown 会 close(fd 0)。如果 gRPC 连接
	// 正好被 accept 到 fd 0，下一次断开就会把连接误关掉。这里把落在
	// std fd 上的连接迁移到高位 fd，保证 gRPC 不受该 bug 影响。
	c = keepConnOffStdFD(c)

	slog.Debug("grpc connection accepted",
		"remote", c.RemoteAddr().String(),
		"local", c.LocalAddr().String(),
		// fd 便于对照适配层日志：若某次 PTP/IP 断开后该 fd 被关闭，
		// 客户端会以 HTTP/2 CONNECT_ERROR(10) 结束连接。
		"fd", connFD(c),
	)

	return c, nil
}

// keepConnOffStdFD 确保连接不会长期占用 fd 0/1/2。
func keepConnOffStdFD(c net.Conn) net.Conn {
	fd := connFD(c)
	if fd < 0 || fd > 2 {
		return c
	}

	moved, err := moveConnOffStdFD(c)
	if err != nil {
		slog.Warn("grpc accept: move connection off std fd failed", "fd", fd, "err", err)
		return c
	}

	// 立即重新占用 fd 0，避免下一次 accept 又拿到它。
	reserveFD0()
	return moved
}

// moveConnOffStdFD 把连接复制到高位 fd，并关闭原来的 std fd 连接。
func moveConnOffStdFD(c net.Conn) (net.Conn, error) {
	sc, ok := c.(syscall.Conn)
	if !ok {
		return nil, errors.New("connection does not expose syscall.Conn")
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return nil, err
	}

	var dupFD int
	var dupErr error
	if err := raw.Control(func(f uintptr) {
		dupFD, dupErr = unix.Dup(int(f))
	}); err != nil {
		return nil, err
	}
	if dupErr != nil {
		return nil, dupErr
	}

	f := os.NewFile(uintptr(dupFD), "grpc-conn")
	moved, err := net.FileConn(f)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	_ = c.Close()
	return moved, nil
}

// reserveFD0 用 /dev/null 占住 fd 0；若 fd 0 已被占用则什么都不做。
func reserveFD0() {
	fd, err := unix.Open(os.DevNull, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return
	}
	if fd != 0 {
		_ = unix.Close(fd)
	}
}

// connFD 返回连接底层的文件描述符；无法获取时返回 -1。
func connFD(c net.Conn) int {
	sc, ok := c.(syscall.Conn)
	if !ok {
		return -1
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return -1
	}
	fd := -1
	if err := raw.Control(func(f uintptr) { fd = int(f) }); err != nil {
		return -1
	}
	return fd
}
