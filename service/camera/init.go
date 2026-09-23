package camera

import (
	"ci.zevix.uk/moment-go/pkg/registry"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

func Init() {
	registry.Register(camerav1.CameraService_ServiceDesc, cameraService{})
}
