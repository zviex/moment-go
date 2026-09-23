package connection

import (
	"ci.zevix.uk/moment-go/pkg/registry"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

func Init() {
	registry.Register(connectionv1.CameraConnectionService_ServiceDesc, connService{})
}
