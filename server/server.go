package server

import (
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

func (s Server) Run() error {
	nl, err := net.Listen(mnet.DefaultNet().Network(), mnet.DefaultNet().Addr())
	if err != nil {
		return err
	}

	defer nl.Close()

	return s.Server.Serve(nl)
}
