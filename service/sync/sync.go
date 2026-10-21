package sync

import (
	"context"

	syncv1 "ci.zevix.uk/moment/moment-protogo/sync/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	syncv1.UnimplementedAutoSyncServiceServer

	recorder *Recorder
}

func New(recorder *Recorder) *Service {
	return &Service{recorder: recorder}
}

func (s *Service) EnableAutoSync(ctx context.Context, req *syncv1.EnableAutoSyncRequest) (*syncv1.EnableAutoSyncResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if err := s.recorder.Enable(); err != nil {
		return nil, status.Errorf(codes.Internal, "autosync: enable: %v", err)
	}

	return &syncv1.EnableAutoSyncResponse{}, nil
}
