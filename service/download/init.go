package download

import (
	"context"

	"ci.zevix.uk/moment-go/pkg/registry"
	downloadv1 "ci.zevix.uk/moment/moment-protogo/download/v1"
)

func Init(ctx context.Context) {
	defaultManager.SetRootContext(ctx)
	registry.Register(downloadv1.DownloadService_ServiceDesc, download{})
}
