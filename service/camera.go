package service

import camerav1 "ci.zevix.uk/moment-go/protogo/camera/v1"

type cameraService struct {
	camerav1.UnimplementedCameraServiceServer
}
