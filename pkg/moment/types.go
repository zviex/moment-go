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
