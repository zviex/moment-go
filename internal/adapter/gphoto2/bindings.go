//go:build cgo

// Package gphoto2 是 libgphoto2 C API 的 cgo 绑定与适配层实现。
//
// libgphoto2 本身支持 Linux / macOS / Android 等平台，libgphoto2-android/include
// 下的头文件同样是跨平台的，因此绑定代码在所有启用 cgo 的目标上编译，
// 不再针对 Android / arm64 做特殊处理。CGO_ENABLED=0 时由 gateway_stub.go
// 提供默认桩实现，保证模块仍可编译与单测。
//
// # 头文件
//
// 编译期统一使用仓库内的头文件（与预编译库同版本、跨平台）：
//
//	libgphoto2-android/include/gphoto2/*.h
//
// 因此宿主机不需要安装 libgphoto2 的开发头文件。
//
// # 链接
//
//   - linux / darwin：链接当前系统安装的 libgphoto2（默认搜索系统库路径，
//     macOS 下额外搜索 /opt/homebrew/lib 与 /usr/local/lib），需要 libgphoto2
//     与 libltdl 的开发包提供 .so / .dylib 链接名；运行期同样使用系统的 camlib / iolib。
//   - android：链接 libgphoto2-android/lib 下随 APK 打包的 .so。
//
// 非标准安装前缀可通过 CGO_CFLAGS / CGO_LDFLAGS 补充。
package gphoto2

/*
#cgo CFLAGS: -I${SRCDIR}/../../../libgphoto2-android/include
#cgo LDFLAGS: -lgphoto2 -lgphoto2_port -lltdl -lm
#cgo darwin LDFLAGS: -L/opt/homebrew/lib -L/usr/local/lib
#cgo android LDFLAGS: -L${SRCDIR}/../../../libgphoto2-android/lib
*/
import "C"
