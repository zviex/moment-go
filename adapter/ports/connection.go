package ports

import (
	"context"

	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
	configv1 "ci.zevix.uk/moment/moment-protogo/config/v1"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

// ConnectionInfo 是一次成功连接后的相机元信息。
type ConnectionInfo struct {
	Model    string
	Driver   string
	PortPath string
}

// ConnectionManager 管理相机会话的连接生命周期。
type ConnectionManager interface {
	Connect(ctx context.Context, target *camerav1.Tartget) (ConnectionInfo, error)
	Disconnect(ctx context.Context) error
}

// TargetResolver 把应用层连接方式解析为后端标准目标。
type TargetResolver interface {
	Resolve(ctx context.Context, opt *connectionv1.ConnectOption) (*camerav1.Tartget, error)
}

// ConfigStore 保存运行期配置。
type ConfigStore interface {
	Set(cfg *configv1.Config)
}
