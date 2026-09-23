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
	"context"
	"log/slog"

	"ci.zevix.uk/moment-go/server"
	"ci.zevix.uk/moment-go/service"
	"ci.zevix.uk/moment/moment-protogo/pkg/logger"
	"ci.zevix.uk/moment/moment-protogo/pkg/logger/sink/std"
)

func main() {
	ctx := context.Background()

	service.Init()

	lg, err := logger.Setup(ctx, logger.Config{
		ServiceName:    "moment-camera",
		ServiceVersion: "v0.1.0",
		Level:          slog.LevelInfo,
	}, logger.WithSink(std.New(std.WithPrettyPrint())))
	if err != nil {
		panic(err)
	}

	defer func() { _ = lg.Shutdown(ctx) }()

	if err := server.DefaultSrv().Run(); err != nil {
		panic(err)
	}
}
