//go:build android && arm64

package gphoto2

/*
#include <stdlib.h>
#include <gphoto2/gphoto2-context.h>
*/
import "C"

import (
	"sync"
	"unsafe"
)

// contextHandlers 接收来自 C 回调的消息，转发给上层日志。
type contextHandlers struct {
	errorFunc   func(string)
	statusFunc  func(string)
	messageFunc func(string)
}

// 全局回调注册表：C 侧的 void* data 指向一个 C 堆上的 int（注册表键），
// 避免把 Go 指针传给 C，规避 cgo 指针传递规则。
var (
	ctxMu   sync.RWMutex
	ctxReg  = map[int]*contextHandlers{}
	ctxNext int
)

// newGPContext 创建 GPContext，注册回调并返回回调数据指针（用于释放）。
func newGPContext(h *contextHandlers) (*C.GPContext, unsafe.Pointer) {
	if h == nil {
		h = &contextHandlers{}
	}
	ctxMu.Lock()
	ctxNext++
	id := ctxNext
	ctxReg[id] = h
	ctxMu.Unlock()

	// 在 C 堆上分配注册表键，供回调反查。
	idPtr := C.malloc(C.size_t(unsafe.Sizeof(C.int(0))))
	if idPtr == nil {
		ctxMu.Lock()
		delete(ctxReg, id)
		ctxMu.Unlock()
		return nil, nil
	}
	*(*C.int)(idPtr) = C.int(id)

	ctx := C.gp_context_new()
	if ctx == nil {
		unregisterContext(id, idPtr)
		return nil, nil
	}
	// Keep the libgphoto2 context alive without registering Go callback hooks.
	// The Android binary does not require context callbacks during normal camera operations,
	// and avoiding exported C->Go callback functions is the most reliable path for cgo
	// under the Android NDK cross toolchain.
	C.gp_context_set_error_func(ctx, nil, nil)
	C.gp_context_set_status_func(ctx, nil, nil)
	C.gp_context_set_message_func(ctx, nil, nil)
	return ctx, idPtr
}

// freeGPContext 释放 GPContext 并注销回调。
func freeGPContext(ctx *C.GPContext, idPtr unsafe.Pointer) {
	if idPtr != nil {
		id := int(*(*C.int)(idPtr))
		unregisterContext(id, idPtr)
	}
	if ctx != nil {
		C.gp_context_unref(ctx)
	}
}

func unregisterContext(id int, idPtr unsafe.Pointer) {
	ctxMu.Lock()
	delete(ctxReg, id)
	ctxMu.Unlock()
	if idPtr != nil {
		C.free(idPtr)
	}
}

// dispatch 根据回调数据反查处理器并调用。
func dispatch(data unsafe.Pointer, invoke func(*contextHandlers)) {
	if data == nil {
		return
	}
	id := int(*(*C.int)(data))
	ctxMu.RLock()
	h, ok := ctxReg[id]
	ctxMu.RUnlock()
	if ok && h != nil {
		invoke(h)
	}
}
