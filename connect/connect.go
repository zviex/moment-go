package connect

import (
	"context"

	"ci.zevix.uk/moment-go/pkg/moment"
	"ci.zevix.uk/moment-go/pool/camera"
	connectionv1 "ci.zevix.uk/moment-go/protogo/connection/v1"
)

var factory = make(map[connectionv1.ConnectionMode]Connecter)

type Connecter interface {
	ConnectType() connectionv1.ConnectionMode
	connect(req *connectionv1.ConnectCameraRequest) moment.ConnectOptions
}

func Open(ctx context.Context, req *connectionv1.ConnectCameraRequest) (*moment.Camera, error) {
	// 当前端退出，但是后端保持活性，还存在连接实例则直接返回

	if cam := camera.Default(); cam != nil {
		return cam, nil
	}

	client, err := moment.New(moment.WithVerboseLog(true))
	if err != nil {
		return nil, err
	}

	cam, err := client.Connect(ctx, factory[req.Mode].connect(req))
	if err != nil {
		return nil, err
	}

	camera.SetDefault(cam)

	return cam, nil
}

func register(c Connecter) {
	factory[c.ConnectType()] = c
}
