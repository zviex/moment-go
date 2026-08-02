//go:build android && arm64

// Package gphoto2 是 libgphoto2 C API 的 cgo 绑定与适配层实现。
// 仅在 Android arm64 上编译（libgphoto2-android 提供 arm64-v8a 动态库）；
// 其他平台/架构由 gateway_stub*.go 提供空实现。
//
// # 链接说明
//
// 仓库内的 libgphoto2-android 目录提供了预编译的 arm64-v8a 库：
//
//	libgphoto2-android/include     # 头文件
//	libgphoto2-android/lib         # 动态库（libgphoto2.so 等）与 camlib/iolib 子目录
//
// 构建时要求 Android NDK 提供的 clang（GOOS=android GOARCH=arm64 CGO_ENABLED=1），
// 例如：
//
//	GOOS=android GOARCH=arm64 CGO_ENABLED=1 CC=$NDK/toolchains/llvm/prebuilt/*/bin/aarch64-linux-android21-clang go build ./...
package gphoto2

/*
#cgo CFLAGS: -I${SRCDIR}/../../../libgphoto2-android/include
#cgo LDFLAGS: -L${SRCDIR}/../../../libgphoto2-android/lib -lgphoto2 -lgphoto2_port -lltdl -lm

#include <gphoto2/gphoto2.h>
#include <gphoto2/gphoto2-camera.h>
#include <gphoto2/gphoto2-context.h>
#include <gphoto2/gphoto2-filesys.h>
#include <gphoto2/gphoto2-file.h>
#include <gphoto2/gphoto2-list.h>
#include <gphoto2/gphoto2-abilities-list.h>
#include <gphoto2/gphoto2-port-info-list.h>
#include <gphoto2/gphoto2-result.h>
#include <gphoto2/gphoto2-widget.h>
#include <stdlib.h>
#include <string.h>

// 复制 CameraFileInfo 中的 MIME 类型。
// 注意：info->file.type 的字段名 type 是 Go 关键字，cgo 无法直接访问，
// 因此通过 C 辅助函数读取。
static void gp_file_info_mime(const CameraFileInfo *info, char *buf, size_t bufsize) {
	if (!info || !buf || bufsize <= 0) return;
	strncpy(buf, info->file.type, bufsize - 1);
	buf[bufsize - 1] = '\0';
}
*/
import "C"
