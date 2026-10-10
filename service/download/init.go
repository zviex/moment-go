package download

import (
	"context"
	"fmt"

	"ci.zevix.uk/moment-go/pkg/registry"
	downloadv1 "ci.zevix.uk/moment/moment-protogo/download/v1"
)

// Init 绑定根 ctx、打开下载历史数据库并注册服务。
// 数据库无法打开时返回错误，由调用方决定是否终止启动。
func Init(ctx context.Context) error {
	s, err := OpenStore(ctx, DefaultDBPath)
	if err != nil {
		return fmt.Errorf("download: open history store: %w", err)
	}
	defaultManager.SetRootContext(ctx)
	defaultManager.SetStore(s)
	registry.Register(downloadv1.DownloadService_ServiceDesc, download{})
	return nil
}
