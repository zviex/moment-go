package adapter

import (
	"context"

	"ci.zevix.uk/moment-go/adapter/ports"
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

// Session 是一次连接绑定的底层会话，只在 Client 与后端之间流转。
// 服务不应依赖该接口：它们只依赖 ports 中的窄能力端口。
type Session interface {
	ports.FileLister
	ports.FileStatter
	ports.FileDownloader
	ports.CameraInfo
	ports.CaptureSource

	Model() string
	Driver() string
	PortPath() string

	Close(ctx context.Context) error
}
