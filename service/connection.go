package service

import camerav1 "ci.zevix.uk/moment-go/protogo/camera/v1"

type connService struct {
	camerav1.UnimplementedCameraConnectionServiceServer
}
