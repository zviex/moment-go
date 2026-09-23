package sync

import (
	"ci.zevix.uk/moment-go/pkg/registry"
	syncv1 "ci.zevix.uk/moment/moment-protogo/sync/v1"
)

func Init() {
	registry.Register(syncv1.AutoSyncService_ServiceDesc, sync{})
}
