package ports

import (
	"context"
	"io"
	"time"

	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

// FileLister 列出相机目录下的直接子目录与文件。
type FileLister interface {
	List(ctx context.Context, folder string) (*camerav1.FileContent, error)
}

// FileStatter 读取相机文件元信息。
type FileStatter interface {
	GetFileInfo(ctx context.Context, folder, name string) (*camerav1.FileEntry, error)
}

// FileDownloader 把相机文件写入 w。
type FileDownloader interface {
	DownloadTo(ctx context.Context, folder, name string, w io.Writer) error
}

// FileBrowser 是浏览相机内容所需的能力组合。
type FileBrowser interface {
	FileLister
	FileStatter
}

// FileReader 是下载相机文件所需的能力组合。
type FileReader interface {
	FileStatter
	FileDownloader
}

// CameraInfo 读取相机聚合信息。
type CameraInfo interface {
	GetInfo(ctx context.Context) (*camerav1.CameraSummary, error)
}

// CaptureEvent 表示一次拍摄产生的新文件。
type CaptureEvent struct {
	URL string
	At  time.Time
}

// CaptureSource 订阅相机拍摄事件。事件流绑定当前连接：
// 连接断开或 ctx 取消时，返回的 channel 会被关闭。
type CaptureSource interface {
	Events(ctx context.Context) (<-chan CaptureEvent, error)
}
