package adapter

import (
	"context"
	"io"
	"time"

	"ci.zevix.uk/moment-go/pkg/task"
	configv1 "ci.zevix.uk/moment/moment-protogo/config/v1"
)

// CameraSource 提供当前已连接的相机会话。
type CameraSource interface {
	Camera() (Session, error)
}

// ConfigStore 保存运行期配置。
type ConfigStore interface {
	Set(cfg *configv1.Config)
}

// Camera 是下载流程真正需要的相机能力。
type Camera interface {
	Session
	GetFileInfo()
	DownloadTo(ctx context.Context, folder, name string, w io.Writer) error
}

type CameraProvider func() (Camera, error)

type Downloader interface {
	Download(ctx context.Context, url string, onProgress func(task.Progress)) error
}

type CaptureEvent struct {
	URL string
	At  time.Time
}

type CaptureSource interface {
	Events(ctx context.Context) (<-chan CaptureEvent, error)
}
