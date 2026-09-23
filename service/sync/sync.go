package sync

import (
	"context"

	syncv1 "ci.zevix.uk/moment/moment-protogo/sync/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type sync struct {
	syncv1.UnimplementedAutoSyncServiceServer
}

// EnableAutoSync 开启自动同步。开启后，每次相机按下快门产生新文件，
// 都会经共享下载行为自动写入 Disk，无需手动下发下载任务。
func (sync) EnableAutoSync(ctx context.Context, req *syncv1.EnableAutoSyncRequest) (*syncv1.EnableAutoSyncResponse, error) {
	if err := defaultRecorder.Enable(); err != nil {
		return nil, status.Errorf(codes.Internal, "autosync: enable: %v", err)
	}
	return &syncv1.EnableAutoSyncResponse{}, nil
}
