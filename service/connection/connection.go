package connection

import (
	"context"

	"ci.zevix.uk/moment-go/adapter/ports"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

type Service struct {
	connectionv1.UnimplementedCameraConnectionServiceServer

	cameras  ports.ConnectionManager
	resolver ports.TargetResolver
	config   ports.ConfigStore
}

func New(cameras ports.ConnectionManager, resolver ports.TargetResolver, config ports.ConfigStore) *Service {
	return &Service{cameras: cameras, resolver: resolver, config: config}
}

func (s *Service) InitlizeCamera(ctx context.Context, req *connectionv1.InitlizeCameraRequest) (*connectionv1.InitlizeCameraResponse, error) {
	s.config.Set(req.GetCfg())
	return nil, nil
}

func (s *Service) ConnectCamera(ctx context.Context, req *connectionv1.ConnectCameraRequest) (*connectionv1.ConnectCameraResponse, error) {
	target, err := s.resolver.Resolve(ctx, req.GetOpt())
	if err != nil {
		return nil, err
	}

	info, err := s.cameras.Connect(ctx, target)
	if err != nil {
		return nil, err
	}

	return &connectionv1.ConnectCameraResponse{
		Model: info.Model,
		Port:  info.PortPath,
	}, nil
}

func (s *Service) DisconnectCamera(ctx context.Context, req *connectionv1.DisconnectCameraRequest) (*connectionv1.DisconnectCameraResponse, error) {
	err := s.cameras.Disconnect(ctx)
	return &connectionv1.DisconnectCameraResponse{}, err
}
