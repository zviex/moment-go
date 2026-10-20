package moment

import (
	"errors"
	"time"

	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

var (
	// ErrBackendUnavailable 当前构建不包含可用后端
	// （未启用 cgo，或未链接到 libgphoto2 动态库）。
	ErrBackendUnavailable = errors.New("libgphoto2 backend is unavailable: build with CGO_ENABLED=1 against libgphoto2")

	// ErrNotConnected 会话未建立或已断开。
	ErrNotConnected = errors.New("camera is not connected")
	// ErrAlreadyConnected 重复连接。
	ErrAlreadyConnected = errors.New("camera is already connected")
	// ErrInvalidState 生命周期状态不允许该操作。
	ErrInvalidState = errors.New("operation not allowed in current lifecycle state")

	// ErrModelNotFound 能力表中找不到指定模型。
	ErrModelNotFound = errors.New("camera model not found")
	// ErrDirectoryNotFound 相机上目录不存在。
	ErrDirectoryNotFound = errors.New("camera directory not found")
	// ErrFileNotFound 相机上文件不存在。
	ErrFileNotFound = errors.New("camera file not found")
	// ErrCameraBusy 相机忙。
	ErrCameraBusy = errors.New("camera is busy")
	// ErrCancelled 操作被取消。
	ErrCancelled = errors.New("operation cancelled")
	// ErrOSFailure 操作系统级错误（如无权限）。
	ErrOSFailure = errors.New("operating system failure")
	// ErrUnsupported 驱动不支持该操作。
	ErrUnsupported = errors.New("operation not supported by camera driver")
)

// IsBackendUnavailable 判断错误是否为“当前构建不含 libgphoto2 后端”。
func IsBackendUnavailable(err error) bool { return errors.Is(err, ErrBackendUnavailable) }

// ConnectOptions 是建立相机连接所需的最小参数集。
type ConnectOptions struct {
	// Type 连接方式：CONNECTION_MODE_AUTO / USB / PTPIP，
	// 留空（UNSPECIFIED）等同自动探测。
	Type connectionv1.ConnectionMode
	// Model 相机模型名；PTP/IP 缺省为 "PTP/IP Camera"。
	Model string
	Host  string
	// Port PTP/IP 端口，0 表示默认 15740。
	Port int
	// USBPort USB 端口路径（形如 "usb:001,002"）。
	USBPort string
	Timeout time.Duration
}

// DetectedCamera 是自动探测到的一台相机。
type DetectedCamera struct {
	Model    string // 相机模型名
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
	BaseDir     string
	Label       string
	Description string
	CapacityKB  int64
	FreeKB      int64
	FreeImages  int64
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
	Model    string
	Driver   string
	PortPath string
	Summary  string
	About    string
	Manual   string
	Storage  []StorageInfo
	Config   *ConfigItem
}

func (c CameraInfo) ProtoMsg() *camerav1.CameraSummary {
	return &camerav1.CameraSummary{
		Model:    c.Model,
		Driver:   c.Driver,
		PortPath: c.PortPath,
		Summary:  c.Summary,
		About:    c.About,
		Manual:   c.Manual,
	}
}

// CameraContent 是一次目录树遍历的结果（预览所有内容）。
type CameraContent struct {
	Folders []Folder
	Files   []File
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
