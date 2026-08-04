package connect

import (
	"context"

	"ci.zevix.uk/moment-go/pkg/moment"
	connectionv1 "ci.zevix.uk/moment-go/protogo/connection/v1"
)

var factory = make(map[connectionv1.ConnectionMode]Connecter)

type Connecter interface {
	ConnectType() connectionv1.ConnectionMode
	connect(req *connectionv1.ConnectCameraRequest) moment.ConnectOptions
}

func Open(ctx context.Context, req *connectionv1.ConnectCameraRequest) (*moment.Camera, error) {
	client, err := moment.New(moment.WithVerboseLog(true))
	if err != nil {
		return nil, err
	}
	return client.Connect(ctx, factory[req.Mode].connect(req))
}

func register(c Connecter) {
	factory[c.ConnectType()] = c
}
