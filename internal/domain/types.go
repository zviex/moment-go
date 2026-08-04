// Package domain 是整洁架构的最内层：领域实体与领域服务，不依赖任何外部实现。
package domain

import (
	"time"

	connectionv1 "ci.zevix.uk/moment-go/protogo/connection/v1"
)

// // ConnectionType 描述与相机的连接方式。
// type ConnectionType int

// const (
// 	// ConnectAuto 自动探测（USB 相机自动检测，取第一个结果）。
// 	ConnectAuto ConnectionType = iota
// 	// ConnectUSB 通过指定 USB 端口连接（形如 "usb:bus,dev" 或 "usb:vid,pid"）。
// 	ConnectUSB
// 	// ConnectPTPIP 通过 PTP/IP 网络协议连接相机。
// 	ConnectPTPIP
// )

// func (t ConnectionType) String() string {
// 	switch t {
// 	case ConnectAuto:
// 		return "auto"
// 	case ConnectUSB:
// 		return "usb"
// 	case ConnectPTPIP:
// 		return "ptpip"
// 	default:
// 		return "unknown"
// 	}
// }

// ConnectOptions 是建立相机连接所需的最小参数集。
type ConnectOptions struct {
	Type connectionv1.ConnectionMode
	// Model 指定相机模型（能力表中的模型名）。
	// PTP/IP 时缺省为 "PTP/IP Camera"；USB 时缺省为自动探测到的模型。
	Model string
	// Host 为 PTP/IP 相机地址（IP 或主机名）。
	Host string
	// Port 为 PTP/IP 端口，0 表示使用默认端口 15740。
	Port int
	// USBPort 为 USB 端口路径，形如 "usb:001,002"。
	USBPort string
	// Timeout 为单次连接操作的超时时间；<=0 表示不超时。
	Timeout time.Duration
}

// DetectedCamera 是自动探测到的一台相机。
type DetectedCamera struct {
	Model    string // 相机模型名（能力表模型名）
	PortPath string // 端口路径，如 "usb:001,002"
}

// Folder 表示相机文件系统中的一个目录。
type Folder struct {
	Name string // 目录名
	Path string // 绝对路径（gphoto2 风格，如 "/store_00010001/DCIM"）
}

// File 表示相机文件系统中的一个文件。
type File struct {
	Name        string // 文件名
	Folder      string // 所在目录的绝对路径
	Size        int64  // 字节数（未知时为 0）
	MimeType    string // MIME 类型
	Width       int    // 图像宽（未知时为 0）
	Height      int    // 图像高（未知时为 0）
	MTime       time.Time
	Downloaded  bool // 是否已下载
	Permissions int  // 访问权限位掩码
}

// StorageInfo 是相机存储（SD 卡等）的信息。
type StorageInfo struct {
	BaseDir     string // 基准目录，通常 "/"
	Label       string // 存储卷标
	Description string // 描述
	CapacityKB  int64  // 总容量（KB）
	FreeKB      int64  // 剩余容量（KB）
	FreeImages  int64  // 剩余可拍张数（相机估算）
}

// ConfigType 是相机配置项（widget）类型。
type ConfigType int

const (
	ConfigWindow  ConfigType = iota // 顶层窗口
	ConfigSection                   // 分组
	ConfigText                      // 文本
	ConfigRange                     // 数值区间（float）
	ConfigToggle                    // 开关（bool/int）
	ConfigRadio                     // 单选
	ConfigMenu                      // 菜单（同单选）
	ConfigButton                    // 按钮
	ConfigDate                      // 日期（int 时间戳）
)

// ConfigItem 是相机配置树中的一个节点。
type ConfigItem struct {
	Name     string
	Label    string
	Info     string
	Type     ConfigType
	Value    any // string / float64 / int，取决于 Type
	Choices  []string
	ReadOnly bool
	Children []*ConfigItem
}

// CameraInfo 是相机信息的聚合结果。
type CameraInfo struct {
	Model    string // 相机模型名
	Driver   string // 驱动（camlib）名，如 "ptp2.so"
	PortPath string // 端口路径
	Summary  string // 相机摘要（固件版本、序列号等，厂商文本）
	About    string // 驱动说明文本
	Manual   string // 驱动手册文本
	Storage  []StorageInfo
	Config   *ConfigItem // 配置树（可能为空树）
}

// State 是相机会话的生命周期状态。
type State int

const (
	StateDisconnected  State = iota // 未连接
	StateConnecting                 // 连接中
	StateConnected                  // 已连接
	StateDisconnecting              // 断开中
	StateError                      // 出错（可再次 Connect）
)

func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateDisconnecting:
		return "disconnecting"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}
