package moment

import (
	"errors"
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

// DetectedCamera 是自动探测到的一台相机。
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

func (c ConfigItem) String() string {
	return ""
}
