package connection

import (
	"context"

	"ci.zevix.uk/moment-go/config"
	"ci.zevix.uk/moment-go/connect"
	"ci.zevix.uk/moment-go/pool/camera"
	configv1 "ci.zevix.uk/moment-go/protogo/config/v1"
	connectionv1 "ci.zevix.uk/moment-go/protogo/connection/v1"
)

type connService struct {
	connectionv1.UnimplementedCameraConnectionServiceServer
}

func (connService) InitlizeCamera(ctx context.Context, req *connectionv1.InitlizeCameraRequest) (*connectionv1.InitlizeCameraResponse, error) {
	config.SetDefault(&configv1.Config{Path: req.GetOpPath()})
	return nil, nil
}

func (connService) ConnectCamera(ctx context.Context, req *connectionv1.ConnectCameraRequest) (*connectionv1.ConnectCameraResponse, error) {
	cam, err := connect.Open(ctx, req)
	if err != nil {
		return nil, err
	}
	camera.SetDefault(cam)
	return nil, nil
}
