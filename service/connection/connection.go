package connection

import (
	"context"

	"ci.zevix.uk/moment-go/adapter"
	"ci.zevix.uk/moment-go/pkg/moment"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

type Service struct {
	connectionv1.UnimplementedCameraConnectionServiceServer
	cameras adapter.CameraConnector
	config  adapter.ConfigStore
}

func New(cameras adapter.CameraConnector, config adapter.ConfigStore) *Service {
	return &Service{cameras: cameras, config: config}
}

func (s *Service) InitlizeCamera(ctx context.Context, req *connectionv1.InitlizeCameraRequest) (*connectionv1.InitlizeCameraResponse, error) {
	s.config.Set(req.GetCfg())
	return nil, nil
}

func (s *Service) ConnectCamera(ctx context.Context, req *connectionv1.ConnectCameraRequest) (*connectionv1.ConnectCameraResponse, error) {
	cam, err := s.cameras.Connect(ctx, connectOptions(req))
	if err != nil {
		return nil, err
	}

	return &connectionv1.ConnectCameraResponse{
		Model: cam.Model(),
		Port:  cam.PortPath(),
	}, nil
}

func (s *Service) DisconnectCamera(ctx context.Context, req *connectionv1.DisconnectCameraRequest) (*connectionv1.DisconnectCameraResponse, error) {
	err := s.cameras.Disconnect(ctx)
	return &connectionv1.DisconnectCameraResponse{}, err
}

func connectOptions(req *connectionv1.ConnectCameraRequest) moment.ConnectOptions {
	if req == nil {
		return moment.ConnectOptions{}
	}

	switch req.GetMode() {
	case connectionv1.ConnectionMode_CONNECTION_MODE_PTPIP:
		return moment.ConnectOptions{
			Type: req.GetMode(),
			Host: req.GetPortPath(),
		}
	case connectionv1.ConnectionMode_CONNECTION_MODE_USB:
		return moment.ConnectOptions{
			Type:    req.GetMode(),
			USBPort: req.GetPortPath(),
		}
	default:
		return moment.ConnectOptions{Type: req.GetMode()}
	}
}
