package connect

import (
	"ci.zevix.uk/moment-go/pkg/moment"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

func init() {
	register(USB{})
}

type USB struct{}

func (USB) ConnectType() connectionv1.ConnectionMode {
	return connectionv1.ConnectionMode_CONNECTION_MODE_USB
}

func (USB) connect(req *connectionv1.ConnectCameraRequest) moment.ConnectOptions {
	return moment.ConnectOptions{
		Type:    connectionv1.ConnectionMode_CONNECTION_MODE_USB,
		USBPort: req.PortPath,
	}
}
