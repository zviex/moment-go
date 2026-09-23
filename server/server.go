package server

import (
	"context"
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
	nlc := net.ListenConfig{}

	nl, err := nlc.Listen(ctx, mnet.DefaultNet().Network(), mnet.DefaultNet().Addr())
	if err != nil {
		return err
	}

	defer nl.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(nl) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		// 根 ctx 取消（收到退出信号）：优雅停机并等待 Serve 返回。
		s.GracefulStop()
		<-errCh
		return nil
	}
}
