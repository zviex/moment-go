package sync

import (
	"context"

	syncv1 "ci.zevix.uk/moment-go/protogo/sync/v1"
)

type sync struct{}

func (s *sync) EnableAutoSync(context.Context, *syncv1.EnableAutoSyncRequest) (*syncv1.EnableAutoSyncResponse, error) {
	panic("unimplemented")
}
