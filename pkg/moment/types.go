// Package moment 是 moment-go 的对外公共 API（delivery 层）。
// 它把内部整洁架构（domain / usecase / adapter）封装为简洁的 Go 接口，
// 围绕相机会话生命周期提供：连接（含 PTP/IP 网络连接）、内容浏览、
// 文件下载与相机信息解析。
package moment

import (
	"errors"
	"time"

	"ci.zevix.uk/moment-go/internal/domain"
	camerav1 "ci.zevix.uk/moment-go/protogo/camera/v1"
	connectionv1 "ci.zevix.uk/moment-go/protogo/connection/v1"
)

// 公共错误哨兵：供调用方做错误分类。
var (
	// ErrUnsupportedPlatform 当前平台不提供 libgphoto2 后端（仅 Android arm64）。
	ErrUnsupportedPlatform = domain.ErrUnsupportedPlatform
	// ErrNotConnected 会话未建立或已断开。
	ErrNotConnected = domain.ErrNotConnected
	// ErrAlreadyConnected 重复连接。
	ErrAlreadyConnected = domain.ErrAlreadyConnected
	// ErrModelNotFound 能力表中找不到指定模型。
	ErrModelNotFound = domain.ErrModelNotFound
	// ErrDirectoryNotFound 相机上目录不存在。
	ErrDirectoryNotFound = domain.ErrDirectoryNotFound
	// ErrFileNotFound 相机上文件不存在。
	ErrFileNotFound = domain.ErrFileNotFound
	// ErrCameraBusy 相机忙。
	ErrCameraBusy = domain.ErrCameraBusy
	// ErrCancelled 操作被取消。
	ErrCancelled = domain.ErrCancelled
)

// IsUnsupported 判断错误是否为“后端在当前平台不可用”。
func IsUnsupported(err error) bool { return errors.Is(err, ErrUnsupportedPlatform) }

// ConnectOptions 是连接参数。
type ConnectOptions struct {
	// Type 连接方式，默认 ConnectPTPIP 之外的语义见各字段。
	Type connectionv1.ConnectionMode
	// Model 相机模型名；PTP/IP 缺省为 "PTP/IP Camera"。
	Model string
	// Host PTP/IP 相机地址（IP 或主机名）。
	Host string
	// Port PTP/IP 端口，0 表示默认 15740。
	Port int
	// USBPort USB 端口路径（形如 "usb:001,002"）。
	USBPort string
	// Timeout 连接超时，<=0 不超时。
	Timeout time.Duration
}

// DetectedCamera 是探测到的相机。
type DetectedCamera struct {
	Model    string
	PortPath string
}

// Folder 是相机上的目录。
type Folder struct {
	Name string
	Path string
}

// File 是相机上的文件。
type File struct {
	Name        string
	Folder      string
	Size        int64
	MimeType    string
	Width       int
	Height      int
	MTime       time.Time
	Downloaded  bool
	Permissions int
}

// StorageInfo 是相机存储信息。
type StorageInfo struct {
	BaseDir     string
	Label       string
	Description string
	CapacityKB  int64
	FreeKB      int64
	FreeImages  int64
}

// ConfigType 是配置项类型。
type ConfigType int

const (
	ConfigWindow ConfigType = iota
	ConfigSection
	ConfigText
	ConfigRange
	ConfigToggle
	ConfigRadio
	ConfigMenu
	ConfigButton
	ConfigDate
)

// ConfigItem 是配置树节点。
type ConfigItem struct {
	Name     string
	Label    string
	Info     string
	Type     ConfigType
	Value    any
	Choices  []string
	ReadOnly bool
	Children []*ConfigItem
}

// CameraInfo 是相机信息聚合。
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

// State 是生命周期状态。
type State int

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
	StateDisconnecting
	StateError
)

func (s State) String() string { return domain.State(s).String() }

// ---- domain 转换 ----

func toDomainOptions(o ConnectOptions) domain.ConnectOptions {
	return domain.ConnectOptions{
		Type:    connectionv1.ConnectionMode(o.Type),
		Model:   o.Model,
		Host:    o.Host,
		Port:    o.Port,
		USBPort: o.USBPort,
		Timeout: o.Timeout,
	}
}

func toDetected(ds []domain.DetectedCamera) []DetectedCamera {
	out := make([]DetectedCamera, len(ds))
	for i, d := range ds {
		out[i] = DetectedCamera{Model: d.Model, PortPath: d.PortPath}
	}
	return out
}

func toFolders(ds []domain.Folder) []Folder {
	out := make([]Folder, len(ds))
	for i, d := range ds {
		out[i] = Folder{Name: d.Name, Path: d.Path}
	}
	return out
}

func toFiles(ds []domain.File) []File {
	out := make([]File, len(ds))
	for i, d := range ds {
		out[i] = toFile(d)
	}
	return out
}

func toFile(d domain.File) File {
	return File{
		Name:        d.Name,
		Folder:      d.Folder,
		Size:        d.Size,
		MimeType:    d.MimeType,
		Width:       d.Width,
		Height:      d.Height,
		MTime:       d.MTime,
		Downloaded:  d.Downloaded,
		Permissions: d.Permissions,
	}
}

func toInfo(d domain.CameraInfo) CameraInfo {
	info := CameraInfo{
		Model:    d.Model,
		Driver:   d.Driver,
		PortPath: d.PortPath,
		Summary:  d.Summary,
		About:    d.About,
		Manual:   d.Manual,
	}
	for _, s := range d.Storage {
		info.Storage = append(info.Storage, StorageInfo{
			BaseDir:     s.BaseDir,
			Label:       s.Label,
			Description: s.Description,
			CapacityKB:  s.CapacityKB,
			FreeKB:      s.FreeKB,
			FreeImages:  s.FreeImages,
		})
	}
	if d.Config != nil {
		info.Config = toConfigItem(d.Config)
	}
	return info
}

func toConfigItem(d *domain.ConfigItem) *ConfigItem {
	if d == nil {
		return nil
	}
	item := &ConfigItem{
		Name:     d.Name,
		Label:    d.Label,
		Info:     d.Info,
		Type:     ConfigType(d.Type),
		Value:    d.Value,
		Choices:  d.Choices,
		ReadOnly: d.ReadOnly,
	}
	for _, c := range d.Children {
		item.Children = append(item.Children, toConfigItem(c))
	}
	return item
}
