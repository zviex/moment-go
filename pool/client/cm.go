package client

import "ci.zevix.uk/moment-go/pkg/moment"

// _defaultCam the singleton obj of Cam
var _defaultCam *moment.Client

func SetDefault(cam *moment.Client) {
	_defaultCam = cam
}

func Default() *moment.Client {
	return _defaultCam
}
