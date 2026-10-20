package server

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"ci.zevix.uk/moment-go/server/mnet"
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
		s.GracefulStop()
		<-errCh
		return nil
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

	slog.Debug("grpc connection accepted",
		"remote", c.RemoteAddr().String(),
		"local", c.LocalAddr().String(),
	)

	return c, nil
}
