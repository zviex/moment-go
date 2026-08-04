package camera

import "ci.zevix.uk/moment-go/pkg/moment"

// _defaultCam the singleton obj of Cam
var _defaultCam = &moment.Camera{}

func SetDefault(cam *moment.Camera) {
	_defaultCam = cam
}

func Default() *moment.Camera {
	return _defaultCam
}
