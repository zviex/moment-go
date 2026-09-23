package camera

import (
	"context"

	"ci.zevix.uk/moment-go/pool/client"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cameraService struct {
	camerav1.UnimplementedCameraServiceServer
}

func (cameraService) GetSummary(ctx context.Context, req *camerav1.GetSummaryRequest) (*camerav1.GetSummaryResponse, error) {
	c, err := client.Default().Camera()
	if err != nil {
		return nil, err
	}

	info, err := c.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &camerav1.GetSummaryResponse{
		Parsed: info.ProtoMsg(),
	}, nil
}

func (cameraService) ListFiles(ctx context.Context, req *camerav1.ListFilesRequest) (*camerav1.ListFilesResponse, error) {
	// 此处返回文件并不返回关于缩略图，防止加载过多缩略图导致接口面板，UI响应迟缓，使用异步加载缩略图数据
	fileInfo := make([]*camerav1.FileEntry, 0)

	c, err := client.Default().Camera()
	if err != nil {
		return nil, err
	}

	files, err := c.ListFiles(ctx, req.Folder)
	if err != nil {
		return nil, err
	}

	for _, v := range files {
		fileInfo = append(fileInfo, &camerav1.FileEntry{
			Name:        v.Name,
			Floder:      v.Folder,
			Size:        v.Size,
			MimeType:    v.MimeType,
			Mtime:       timestamppb.New(v.MTime),
			Width:       int32(v.Width),
			Height:      int32(v.Height),
			Downloaded:  v.Downloaded,
			Permissions: int32(v.Permissions),
		})
	}

	return &camerav1.ListFilesResponse{
		Folder:  req.Folder,
		Entries: fileInfo,
	}, nil
}

func (cameraService) ListFolders(ctx context.Context, req *camerav1.ListFoldersRequest) (*camerav1.ListFoldersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListFolders not implemented")
}
