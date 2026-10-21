package adapter

import (
	"context"
	"io"

	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

type DetectedCamera struct {
	Model        string // 相机模型名
	PortPath     string // 端口路径，如 "usb:001,002"
	USBVendorID  uint32 // USB Vendor ID（非 USB 设备或未知时为 0）
	USBProductID uint32 // USB Product ID（非 USB 设备或未知时为 0）
}

// ConnectFunc 是后端提供的标准化连接入口：后端只接收已解析好的 Target，
// 不感知自动探测 / USB / PTP/IP 等应用层连接方式。
type ConnectFunc func(ctx context.Context, target *camerav1.Tartget) (Session, error)

type Discoverer interface {
	Detect(ctx context.Context) ([]DetectedCamera, error)
}

// ReadOnlyFS 抽象相机上的只读文件系统：
// 列出目录、列出文件、读取文件信息与打开文件内容。
type ReadOnlyFS interface {
	List(ctx context.Context, folder string) ([]*camerav1.FileContent, error)
	Open(ctx context.Context, folder, name string) (io.ReadCloser, error)
}

// Session 是已建立连接上的相机会话：只读文件系统 + 相机元信息 + 生命周期。
type Session interface {
	ReadOnlyFS

	Model() string
	Driver() string
	PortPath() string

	GetInfo(ctx context.Context) (*camerav1.CameraSummary, error)
	Close(ctx context.Context) error
}
