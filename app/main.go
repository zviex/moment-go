// moment-go demo：演示围绕生命周期的四大用例。
//
// 本地（非 Android）运行时，libgphoto2 后端不可用（stub 返回
// ErrUnsupportedPlatform），但仍可看到完整的 API 与生命周期流程。
// 在 Android (arm64) 上构建后连接真实相机：
//
//	GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
//	CC=$NDK/toolchains/llvm/prebuilt/*/bin/aarch64-linux-android21-clang \
//	go build -o app_demo ./app
package main

import (
	"context"
	"fmt"
	"os"

	"ci.zevix.uk/moment-go/pkg/moment"
)

func main() {
	ctx := context.Background()

	// 1. 创建客户端。Android 上需解压 .so 后传入布局根目录：
	//    client, err := moment.New(moment.WithLibraryRoot(filesDir), moment.WithVerboseLog(true))
	client, err := moment.New(moment.WithVerboseLog(true))
	must(err)

	fmt.Println("state:", client.State())

	// 2. 连接：PTP/IP 网络连接（相机 WiFi/IP 模式）。
	cam, err := client.Connect(ctx, moment.ConnectOptions{
		Type: moment.ConnectPTPIP,
		Host: "192.168.1.100",
	})
	if err != nil {
		// 本地桩环境必然走到这里，属预期。
		if moment.IsUnsupported(err) {
			fmt.Println("[demo] libgphoto2 backend unavailable on this platform (expected locally)")
			_ = client.Close()
			return
		}
		must(err)
	}
	defer cam.Close(ctx)

	fmt.Printf("connected: model=%s driver=%s port=%s\n", cam.Model(), cam.Driver(), cam.PortPath())
	fmt.Println("state:", client.State())

	// 3. 预览所有相机内容（递归遍历目录树）。
	content, err := cam.ListTree(ctx)
	must(err)
	fmt.Printf("folders=%d files=%d\n", len(content.Folders), len(content.Files))

	// 4. 下载第一个文件到本地。
	if len(content.Files) > 0 {
		f := content.Files[0]
		data, err := cam.Download(ctx, f.Folder, f.Name)
		must(err)
		fmt.Printf("downloaded %s (%d bytes)\n", f.Name, len(data))
	}

	// 5. 解析相机信息。
	info, err := cam.GetInfo(ctx)
	must(err)
	fmt.Printf("summary: %s\n", info.Summary)

	// 6. 断开连接。
	must(client.Disconnect(ctx))
	fmt.Println("state:", client.State())

	_ = client.Close()
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
