// 本地（非 Android）运行时，libgphoto2 后端不可用（stub 返回
// ErrUnsupportedPlatform），但仍可看到完整的 API 与生命周期流程。
// 在 Android (arm64) 上构建后连接真实相机：
//
//	GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
//	CC=$NDK/toolchains/llvm/prebuilt/*/bin/aarch64-linux-android21-clang \
//	go build -o app_demo ./app
package main

import "fmt"

func main() {
	fmt.Println("hello world")
}
