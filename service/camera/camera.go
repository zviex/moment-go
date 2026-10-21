package camera

import (
	"context"

	"ci.zevix.uk/moment-go/adapter"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	camerav1.UnimplementedCameraServiceServer

	cameras adapter.CameraSource
}

func New(cameras adapter.CameraSource) *Service {
	return &Service{cameras: cameras}
}

func (s *Service) GetSummary(ctx context.Context, req *camerav1.GetSummaryRequest) (*camerav1.GetSummaryResponse, error) {
	c, err := s.cameras.Camera()
	if err != nil {
		return nil, err
	}

	info, err := c.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &camerav1.GetSummaryResponse{
		Parsed: info,
	}, nil
}

func (s *Service) ListFsFolder(context.Context, *camerav1.ListFsFolderRequest) (*camerav1.ListFsFolderResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListFsFolder not implemented")
}
