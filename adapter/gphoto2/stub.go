//go:build !cgo

package gphoto2

import (
	"context"
	"log/slog"

	"ci.zevix.uk/moment-go/pkg/moment"
)

// Client 是未启用 cgo（CGO_ENABLED=0）时的桩后端。
//
// 桩不依赖 libgphoto2，行为等价于“没有接上任何相机”：
//   - Detect 返回空结果，只记录一条告警；
//   - Connect 返回 moment.ErrBackendUnavailable，提示用 CGO_ENABLED=1 重新构建。
//
// 这样在缺少 C 工具链或 libgphoto2 的环境里，模块依旧可以编译、单测。
type Client struct{}

// New 创建桩后端。
func New(_ ...Option) (*Client, error) {
	slog.Warn("libgphoto2 backend unavailable: built without cgo (CGO_ENABLED=0)")
	return &Client{}, nil
}

// Close 无需释放资源。
func (g *Client) Close() error { return nil }

// Detect 在无 cgo 构建下没有可探测的相机，返回空结果。
func (g *Client) Detect(ctx context.Context) ([]moment.DetectedCamera, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	slog.Warn("libgphoto2 backend unavailable: no camera can be detected")
	return nil, nil
}

// Connect 在无 cgo 构建下不可用。
func (g *Client) Connect(_ context.Context, _ moment.Target) (moment.Session, error) {
	slog.Warn("libgphoto2 backend unavailable: rebuild with CGO_ENABLED=1")
	return nil, moment.ErrBackendUnavailable
}
