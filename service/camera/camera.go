package camera

import (
	"context"
	"path"

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

func (s *Service) ListFiles(ctx context.Context, req *camerav1.ListFilesRequest) (*camerav1.ListFilesResponse, error) {
	folder := req.GetFolder()
	contents, err := s.files.List(ctx, folder)
	if err != nil {
		return nil, err
	}

	var entries []*camerav1.FileEntry
	for _, content := range contents {
		entries = append(entries, content.GetFiles()...)
	}

	return &camerav1.ListFilesResponse{
		Folder:  folder,
		Entries: entries,
	}, nil
}

func (s *Service) ListFolders(ctx context.Context, req *camerav1.ListFoldersRequest) (*camerav1.ListFoldersResponse, error) {
	folder := req.GetFolder()
	contents, err := s.files.List(ctx, folder)
	if err != nil {
		return nil, err
	}

	var entries []*camerav1.FileEntry
	for _, content := range contents {
		for _, dir := range content.GetFolders() {
			entries = append(entries, &camerav1.FileEntry{
				Name:   path.Base(dir),
				Floder: dir,
			})
		}
	}

	return &camerav1.ListFoldersResponse{
		Folder:  folder,
		Entries: entries,
	}, nil
}

func (s *Service) ListFsFolder(ctx context.Context, req *camerav1.ListFsFolderRequest) (*camerav1.ListFsFolderResponse, error) {
	contents, err := s.files.List(ctx, req.GetFolder())
	if err != nil {
		return nil, err
	}
	if len(contents) == 0 {
		return &camerav1.ListFsFolderResponse{}, nil
	}
	return &camerav1.ListFsFolderResponse{Data: contents[0]}, nil
}
