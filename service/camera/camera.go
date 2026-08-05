package camera

import (
	"context"

	"ci.zevix.uk/moment-go/pool/client"
	camerav1 "ci.zevix.uk/moment-go/protogo/camera/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	}, status.Error(codes.Unimplemented, "method GetSummary not implemented")
}

func (cameraService) GetAbilities(ctx context.Context, req *camerav1.GetAbilitiesRequest) (*camerav1.GetAbilitiesResponse, error) {
	c, err := client.Default().Camera()
	if err != nil {
		return nil, err
	}

	return nil, status.Error(codes.Unimplemented, "method GetAbilities not implemented")
}

func (cameraService) ListFiles(ctx context.Context, req *camerav1.ListFilesRequest) (*camerav1.ListFilesResponse, error) {
	// 此处返回文件并不返回关于缩略图，防止加载过多缩略图导致接口面板，UI响应迟缓，使用异步加载缩略图数据
	c, err := client.Default().Camera()
	if err != nil {
		return nil, err
	}

	files, err := c.ListFiles(ctx, "")

	for _, v := range files {
	}

	return nil, status.Error(codes.Unimplemented, "method ListFiles not implemented")
}

func (cameraService) ListFolders(ctx context.Context, req *camerav1.ListFoldersRequest) (*camerav1.ListFoldersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListFolders not implemented")
}

func (cameraService) GetFileInfo(ctx context.Context, req *camerav1.GetFileInfoRequest) (*camerav1.GetFileInfoResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetFileInfo not implemented")
}

func (cameraService) GetFile(ctx context.Context, req *camerav1.GetFileRequest) (*camerav1.GetFileResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetFile not implemented")
}
