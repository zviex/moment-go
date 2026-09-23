package connection

import (
	"context"
	"errors"

	"ci.zevix.uk/moment-go/config"
	"ci.zevix.uk/moment-go/connect"
	"ci.zevix.uk/moment-go/pool/client"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

type connService struct {
	connectionv1.UnimplementedCameraConnectionServiceServer
}

func (connService) InitlizeCamera(ctx context.Context, req *connectionv1.InitlizeCameraRequest) (*connectionv1.InitlizeCameraResponse, error) {
	config.SetDefault(req.Cfg)
	return nil, nil
}

func (connService) ConnectCamera(ctx context.Context, req *connectionv1.ConnectCameraRequest) (*connectionv1.ConnectCameraResponse, error) {
	cam, err := connect.Open(ctx, req)
	if err != nil {
		return nil, err
	}

	return &connectionv1.ConnectCameraResponse{
		Model: cam.Model(),
		Port:  cam.PortPath(),
	}, nil
}

func (connService) DisconnectCamera(ctx context.Context, req *connectionv1.DisconnectCameraRequest) (*connectionv1.DisconnectCameraResponse, error) {
	cl := client.Default()
	if cl == nil {
		return nil, errors.New("cam has disconnect")
	}

	err := cl.Disconnect(ctx)

	return &connectionv1.DisconnectCameraResponse{}, err
}
