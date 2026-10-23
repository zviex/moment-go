package camera

import (
	"context"

	"ci.zevix.uk/moment-go/adapter/ports"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

type Service struct {
	camerav1.UnimplementedCameraServiceServer

	info  ports.CameraInfo
	files ports.FileBrowser
}

func New(info ports.CameraInfo, files ports.FileBrowser) *Service {
	return &Service{info: info, files: files}
}

func (s *Service) GetSummary(ctx context.Context, req *camerav1.GetSummaryRequest) (*camerav1.GetSummaryResponse, error) {
	info, err := s.info.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &camerav1.GetSummaryResponse{
		Parsed: info,
	}, nil
}

func (s *Service) ListFsFolder(ctx context.Context, req *camerav1.ListFsFolderRequest) (*camerav1.ListFsFolderResponse, error) {
	contents, err := s.files.List(ctx, req.GetFolder())
	if err != nil {
		return nil, err
	}

	return &camerav1.ListFsFolderResponse{Data: contents}, nil
}
