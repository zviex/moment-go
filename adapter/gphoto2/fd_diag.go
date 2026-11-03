//go:build cgo

package gphoto2

import (
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/sys/unix"
)

// fd0State 描述进程 fd 0 的状态，用于排查 libgphoto2 PTP/IP 断开时
// 误关进程 fd（上游 2.5.34 的 ptp_ptpip_disconnect() 会 close(jpgfd)，
// 而非富士 PTP/IP 相机上 jpgfd 恒为 0）的问题。
type fd0State struct {
	open   bool
	target string
}

// captureFD0 采样当前 fd 0 是否有效以及它指向什么。
// /proc/self/fd 在 Linux/Android 可用；macOS 上退回 /dev/fd。
func captureFD0() fd0State {
	st := fd0State{target: "-"}
	if _, err := unix.FcntlInt(0, unix.F_GETFD, 0); err == nil {
		st.open = true
	}
	for _, p := range []string{"/proc/self/fd/0", "/dev/fd/0"} {
		if target, err := os.Readlink(p); err == nil && target != "" {
			st.target = target
			break
		}
	}
	return st
}

// logFD0 以 debug 级别记录当前 fd 0 状态。
func logFD0(stage string, extra ...any) {
	st := captureFD0()
	args := []any{"stage", stage, "fd0_open", st.open, "fd0", st.target}
	args = append(args, extra...)
	slog.Debug("gphoto2 fd state", args...)
}

// warnIfFD0Closed 在 fd 0 被意外关闭时输出警告。这是 libgphoto2 2.5.34
// PTP/IP 断开路径 close(jpgfd=0) 的直接证据；fd 0 被释放后可能被 gRPC
// 的 unix socket 复用，导致下次断开时客户端收到 HTTP/2 CONNECT_ERROR(10)。
func warnIfFD0Closed(stage string, extra ...any) {
	st := captureFD0()
	if st.open {
		return
	}
	args := []any{"stage", stage}
	args = append(args, extra...)
	slog.Warn("gphoto2: process fd 0 was closed during camera teardown "+
		"(mitigated by fd guard); libgphoto2 PTP/IP close(jpgfd=0) is the cause", args...)
}

// openFDSet 返回当前进程打开的全部 fd 集合。
// 仅在诊断路径调用（断开耗时远大于扫描 1024 个 fd 的系统调用开销）。
func openFDSet() map[int]struct{} {
	set := make(map[int]struct{}, 64)
	for fd := 0; fd < 1024; fd++ {
		if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err == nil {
			set[fd] = struct{}{}
		}
	}
	return set
}

// diffFDSet 返回从 before 到 after 消失与新增的 fd。
func diffFDSet(before, after map[int]struct{}) (closed, opened []int) {
	for fd := range before {
		if _, ok := after[fd]; !ok {
			closed = append(closed, fd)
		}
	}
	for fd := range after {
		if _, ok := before[fd]; !ok {
			opened = append(opened, fd)
		}
	}
	sortInts(closed)
	sortInts(opened)
	return closed, opened
}

func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// logFDDiff 记录两个采样点之间 fd 集合的变化，便于定位被误关的 fd
// （例如 gRPC 的 unix socket fd）。
func logFDDiff(stage string, before, after map[int]struct{}) {
	closed, opened := diffFDSet(before, after)
	slog.Debug("gphoto2 fd diff",
		"stage", stage,
		"closed", fmt.Sprint(closed),
		"opened", fmt.Sprint(opened),
	)
}
