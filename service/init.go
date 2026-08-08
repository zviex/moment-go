package service

import (
	"ci.zevix.uk/moment-go/service/camera"
	"ci.zevix.uk/moment-go/service/connection"
	"ci.zevix.uk/moment-go/service/download"
	"ci.zevix.uk/moment-go/service/sync"
)

func Init() {
	camera.Init()
	connection.Init()
	download.Init()
	sync.Init()
}
