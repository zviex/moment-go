//go:build cgo

package gphoto2

import (
	"testing"

	"golang.org/x/sys/unix"
)

// TestGuardFD0RestoresAfterClose 模拟 libgphoto2 2.5.34 在 PTP/IP 断开时
// 执行的 close(0)，验证 guardFD0 会把进程 fd 0 恢复回来，避免 fd 0 被
// 后续 gRPC accept 复用。
func TestGuardFD0RestoresAfterClose(t *testing.T) {
	if !captureFD0().open {
		t.Fatal("precondition: fd 0 should be open in a Go process")
	}

	for i := 0; i < 3; i++ {
		guardFD0(func() {
			// 模拟 ptp_ptpip_disconnect(): close(jpgfd)，jpgfd 恒为 0。
			if err := unix.Close(0); err != nil {
				t.Fatalf("iteration %d: close(0): %v", i, err)
			}
			if captureFD0().open {
				t.Fatalf("iteration %d: fd 0 should be closed inside guard", i)
			}
		})

		if !captureFD0().open {
			t.Fatalf("iteration %d: fd 0 was not restored after guard", i)
		}
	}
}
