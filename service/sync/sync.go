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

func (sync) EnableAutoSync(ctx context.Context, req *syncv1.EnableAutoSyncRequest) (*syncv1.EnableAutoSyncResponse, error) {
	if err := defaultRecorder.Enable(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "autosync: enable: %v", err)
	}

	return &syncv1.EnableAutoSyncResponse{}, nil
}
