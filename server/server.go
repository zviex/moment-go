package server

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"ci.zevix.uk/moment-go/server/mnet"
	"google.golang.org/grpc"
)

var _default = NewServer()

func DefaultSrv() Server {
	return _default
}

type Server struct {
	*grpc.Server
}

func NewServer() Server {
	return Server{
		Server: grpc.NewServer(),
	}
}

func (s Server) Run(ctx context.Context) error {
	netName := mnet.DefaultNet().Network()
	addr := mnet.DefaultNet().Addr()

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
		// 根 ctx 取消（收到退出信号）：优雅停机并等待 Serve 返回。
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
