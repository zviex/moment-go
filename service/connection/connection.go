package connection

import (
	"context"

	connectionv1 "ci.zevix.uk/moment-go/protogo/connection/v1"
)

type connService struct {
	connectionv1.UnimplementedCameraConnectionServiceServer
}

func (connService) InitlizeCamera(context.Context, *connectionv1.InitlizeCameraRequest) (*connectionv1.InitlizeCameraResponse, error) {
	return nil, nil
}

func (connService) ConnectCamera(context.Context, *connectionv1.ConnectCameraRequest) (*connectionv1.ConnectCameraResponse, error) {
	return nil, nil
}
