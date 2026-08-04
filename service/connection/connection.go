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
	// 当前端退出，但是后端保持活性，还存在连接实例则直接返回
	cam, err := connect.Open(ctx, req)
	if err != nil {
		return nil, err
	}
	camera.SetDefault(cam)

	c := camera.Default()

	resp := connectionv1.ConnectCameraResponse{
		Model: c.Model(),
		Port:  c.PortPath(),
	}

	return &resp, nil
}
