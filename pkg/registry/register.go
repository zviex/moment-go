package registry

import (
	"ci.zevix.uk/moment-go/server"
	"google.golang.org/grpc"
)

func Register(a grpc.ServiceDesc, impl any) {
	server.DefaultSrv().RegisterService(&a, impl)
}
