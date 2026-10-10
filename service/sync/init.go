package sync

import (
	"context"

	"ci.zevix.uk/moment-go/pkg/registry"
	syncv1 "ci.zevix.uk/moment/moment-protogo/sync/v1"
)

func Init(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	defaultRecorder = NewDefaultRecorder(ctx, nil)

	registry.Register(syncv1.AutoSyncService_ServiceDesc, sync{})
}
