//go:build !cgo

package gphoto2

import (
	"context"

	"ci.zevix.uk/moment-go/internal/domain"
	"ci.zevix.uk/moment-go/internal/logging"
)

// Gateway 是未启用 cgo（CGO_ENABLED=0）时的默认桩后端。
//
// 桩不依赖 libgphoto2，行为等价于"没有接上任何相机"：
//
//   - Detect 返回空结果，只记录一条告警；
//   - Connect 返回 domain.ErrBackendUnavailable，提示用 CGO_ENABLED=1 重新构建。
//
// 这样在缺少 C 工具链或 libgphoto2 的环境里，模块依旧可以编译、单测，
// 并演示完整的 API 与生命周期流程。
type Gateway struct {
	logger logging.Logger
}

// NewGateway 创建桩网关。
func NewGateway(logger logging.Logger) (*Gateway, error) {
	if logger == nil {
		logger = logging.Nop{}
	}
	logger.Warnf("libgphoto2 backend unavailable: built without cgo (CGO_ENABLED=0)")
	return &Gateway{logger: logger}, nil
}

// Close 无需释放资源。
func (g *Gateway) Close() error { return nil }

// Detect 在无 cgo 构建下没有可探测的相机，返回空结果。
func (g *Gateway) Detect(ctx context.Context) ([]domain.DetectedCamera, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	g.logger.Warnf("libgphoto2 backend unavailable: no camera can be detected")
	return nil, nil
}

// Connect 在无 cgo 构建下不可用。
func (g *Gateway) Connect(_ context.Context, _ domain.ConnectOptions) (domain.CameraSession, error) {
	g.logger.Warnf("libgphoto2 backend unavailable: rebuild with CGO_ENABLED=1")
	return nil, domain.ErrBackendUnavailable
}
