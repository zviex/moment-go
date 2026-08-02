package domain

import (
	"context"
	"io"
)

// Gateway 是领域层定义的端口（接口）：描述"连接相机"这一能力，
// 由适配层（internal/adapter）提供具体实现，领域层与用例层只依赖此接口。
type Gateway interface {
	// Detect 自动探测可用相机（USB），并列出可用端口。
	Detect(ctx context.Context) ([]DetectedCamera, error)
	// Connect 按选项建立相机连接，返回一个可用的相机会话。
	Connect(ctx context.Context, opts ConnectOptions) (CameraSession, error)
	// Close 释放网关持有的底层资源（GPContext 等）。
	Close() error
}

// CameraSession 描述一个已连接相机上的会话能力，同样属于端口定义。
type CameraSession interface {
	// Model 返回相机模型名。
	Model() string
	// Driver 返回驱动（camlib）名。
	Driver() string
	// PortPath 返回连接端口路径。
	PortPath() string

	// ListFolders 列出 folder 下的子目录。
	ListFolders(ctx context.Context, folder string) ([]Folder, error)
	// ListFiles 列出 folder 下的文件（含文件信息）。
	ListFiles(ctx context.Context, folder string) ([]File, error)
	// GetFileInfo 查询单个文件的详细信息。
	GetFileInfo(ctx context.Context, folder, name string) (File, error)

	// Download 下载文件完整内容。
	Download(ctx context.Context, folder, name string) ([]byte, error)
	// DownloadTo 将文件内容写入 w。
	DownloadTo(ctx context.Context, folder, name string, w io.Writer) error

	// GetInfo 聚合相机信息：摘要 / 驱动说明 / 存储 / 配置树。
	GetInfo(ctx context.Context) (CameraInfo, error)

	// Close 关闭会话（断开相机连接并释放资源）。
	Close(ctx context.Context) error
}
