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
	"os"
	"os/signal"
	"syscall"

	"ci.zevix.uk/moment-go/server"
	"ci.zevix.uk/moment-go/server/mnet"
	"ci.zevix.uk/moment-go/service"
	"ci.zevix.uk/moment/moment-protogo/pkg/logger"
	"ci.zevix.uk/moment/moment-protogo/pkg/logger/sink/victorialogs"
)

func main() {
	// 根 ctx 由 SIGINT/SIGTERM 触发取消，贯穿服务初始化与服务运行，
	// 使自动同步、下载任务等长生命周期协程随进程退出而终止。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	vl, err := victorialogs.New("http://logs.home.arpa:9740")
	if err != nil {
		panic(err)
	}

	lg, err := logger.Setup(ctx, logger.Config{
		ServiceName:    "moment-camera",
		ServiceVersion: "v0.1.0",
		Level:          slog.LevelDebug,
	}, logger.WithSink(vl))
	if err != nil {
		panic(err)
	}

	defer func() { _ = lg.Shutdown(context.Background()) }()

	if err := service.Init(ctx); err != nil {
		panic(err)
	}
	mnet.Init()

	if err := server.DefaultSrv().Run(ctx); err != nil {
		panic(err)
	}
}
