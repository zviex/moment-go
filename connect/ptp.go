package connect

import (
	"ci.zevix.uk/moment-go/pkg/moment"
	connectionv1 "ci.zevix.uk/moment-go/protogo/connection/v1"
)

func init() {
	register(PTP{})
}

type PTP struct{}

func (PTP) ConnectType() connectionv1.ConnectionMode {
	return connectionv1.ConnectionMode_CONNECTION_MODE_PTPIP
}

func (p PTP) connect(req *connectionv1.ConnectCameraRequest) moment.ConnectOptions {
	return moment.ConnectOptions{
		Type: p.ConnectType(),
		Host: req.PortPath,
	}
}
