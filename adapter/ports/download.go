package ports

import (
	"context"

	"ci.zevix.uk/moment-go/pkg/task"
)

// Downloader 是从相机下载单个文件并落盘的应用动作。
type Downloader interface {
	Download(ctx context.Context, url string, onProgress func(task.Progress)) error
}
