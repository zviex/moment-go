//go:build !android

package gphoto2

import (
	"context"

	"ci.zevix.uk/moment-go/internal/domain"
	"ci.zevix.uk/moment-go/internal/logging"
)

// Gateway 是 libgphoto2 适配器的桩实现。
// libgphoto2-android 提供的是 Android arm64 的动态库，因此在其他平台上
// 所有操作均返回 domain.ErrUnsupportedPlatform，保证模块可编译、可演示
// API 与生命周期流程。
type Gateway struct{}

// NewGateway 创建桩网关。
func NewGateway(_ logging.Logger) (*Gateway, error) {
	return &Gateway{}, nil
}

// Close 无需释放资源。
func (g *Gateway) Close() error { return nil }

// Detect 在当前平台不可用。
func (g *Gateway) Detect(_ context.Context) ([]domain.DetectedCamera, error) {
	return nil, domain.ErrUnsupportedPlatform
}

// Connect 在当前平台不可用。
func (g *Gateway) Connect(_ context.Context, _ domain.ConnectOptions) (domain.CameraSession, error) {
	return nil, domain.ErrUnsupportedPlatform
}
