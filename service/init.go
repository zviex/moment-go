package service

import (
	"context"

	"ci.zevix.uk/moment-go/service/camera"
	"ci.zevix.uk/moment-go/service/connection"
	"ci.zevix.uk/moment-go/service/download"
	"ci.zevix.uk/moment-go/service/sync"
)

// Init 注册各服务并把应用级根 ctx 绑定到需要长生命周期协程的服务
// （自动同步、下载任务），使根 ctx 取消时这些协程随之终止。
func Init(ctx context.Context) {
	camera.Init()
	connection.Init()
	sync.Init(ctx)
	download.Init(ctx)
}
