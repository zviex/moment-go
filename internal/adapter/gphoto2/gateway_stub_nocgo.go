//go:build android && !arm64

package gphoto2

import (
	"context"

	"ci.zevix.uk/moment-go/internal/domain"
	"ci.zevix.uk/moment-go/internal/logging"
)

// Gateway 是 Android 非 arm64 架构下的桩实现。
// libgphoto2-android 仅提供 arm64-v8a 动态库，因此其他架构返回
// domain.ErrUnsupportedPlatform。
type Gateway struct{}

// NewGateway 创建桩网关。
func NewGateway(_ logging.Logger) (*Gateway, error) {
	return &Gateway{}, nil
}

// Close 无需释放资源。
func (g *Gateway) Close() error { return nil }

// Detect 在当前架构不可用。
func (g *Gateway) Detect(_ context.Context) ([]domain.DetectedCamera, error) {
	return nil, domain.ErrUnsupportedPlatform
}

// Connect 在当前架构不可用。
func (g *Gateway) Connect(_ context.Context, _ domain.ConnectOptions) (domain.CameraSession, error) {
	return nil, domain.ErrUnsupportedPlatform
}
