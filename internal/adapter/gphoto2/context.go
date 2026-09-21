//go:build cgo

package gphoto2

/*
#include <stdlib.h>
#include <gphoto2/gphoto2-context.h>
*/
import "C"

import "errors"

// newGPContext 创建 libgphoto2 的 GPContext。
//
// 这里不注册 C→Go 回调钩子：正常相机操作不依赖回调，而不导出 C→Go 回调函数可以让
// 交叉编译（含 Android NDK）下的 cgo 更稳定。若后续要把 libgphoto2 的错误 / 状态
// 消息转发到 logger，可在本文件用 //export 定义回调并注册到对应 set_*_func。
func newGPContext() (*C.GPContext, error) {
	ctx := C.gp_context_new()
	if ctx == nil {
		return nil, errors.New("failed to create gphoto2 GPContext")
	}
	return ctx, nil
}

// freeGPContext 释放 GPContext 引用。
func freeGPContext(ctx *C.GPContext) {
	if ctx != nil {
		C.gp_context_unref(ctx)
	}
}
