// 默认（CGO_ENABLED=1）会链接系统的 libgphoto2，可直接连接真实相机；
// CGO_ENABLED=0 时使用桩后端（Connect 返回 moment.ErrBackendUnavailable），
// 仍可看到完整的 API 与生命周期流程。
//
//	# 宿主（Linux / macOS）
//	go build -o app_demo ./app
//
//	# Android arm64（使用随包 .so）
//	GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
//	CC=$NDK/toolchains/llvm/prebuilt/*/bin/aarch64-linux-android21-clang \
//	go build -o app_demo ./app
package main

import (
	"ci.zevix.uk/moment-go/server"
	"ci.zevix.uk/moment-go/service"
)

func main() {
	service.Init()
	if err := server.DefaultSrv().Run(); err != nil {
		panic(err)
	}
}
