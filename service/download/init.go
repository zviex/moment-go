package download

import (
	"ci.zevix.uk/moment-go/pkg/registry"
	downloadv1 "ci.zevix.uk/moment-go/protogo/download/v1"
)

func Init() {
	registry.Register(downloadv1.DownloadService_ServiceDesc, download{})
}
