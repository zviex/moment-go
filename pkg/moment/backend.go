package moment

import (
	"context"
	"io"
)

// ConnectFunc 是后端提供的标准化连接入口：后端只接收已解析好的 Target，
// 不感知自动探测 / USB / PTP/IP 等应用层连接方式。
type ConnectFunc func(ctx context.Context, target Target) (Session, error)

type Discoverer interface {
	Detect(ctx context.Context) ([]DetectedCamera, error)
}

// ReadOnlyFS 抽象相机上的只读文件系统：
// 列出目录、列出文件、读取文件信息与打开文件内容。
type ReadOnlyFS interface {
	ListFolders(ctx context.Context, folder string) ([]Folder, error)
	ListFiles(ctx context.Context, folder string) ([]File, error)
	GetFileInfo(ctx context.Context, folder, name string) (File, error)
	Open(ctx context.Context, folder, name string) (io.ReadCloser, error)
}

// Session 是已建立连接上的相机会话：只读文件系统 + 相机元信息 + 生命周期。
type Session interface {
	ReadOnlyFS

	Model() string
	Driver() string
	PortPath() string

	GetInfo(ctx context.Context) (CameraInfo, error)
	Close(ctx context.Context) error
}
