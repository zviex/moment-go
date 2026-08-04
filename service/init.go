package service

import (
	"ci.zevix.uk/moment-go/service/camera"
	"ci.zevix.uk/moment-go/service/connection"
)

func Init() {
	camera.Init()
	connection.Init()
}
