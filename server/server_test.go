package server

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// TestMoveConnOffStdFD 验证连接被复制到高位 fd，从而不会因为占用 std fd
// 而被 libgphoto2 的 PTP/IP teardown 误关。
func TestMoveConnOffStdFD(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "grpc.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	dialErr := make(chan error, 1)
	go func() {
		c, err := net.Dial("unix", sock)
		if err != nil {
			dialErr <- err
			return
		}
		defer c.Close()
		// 保持连接，直到服务端完成 accept / move。
		_, _ = c.Write([]byte("x"))
		dialErr <- nil
	}()

	accepted, err := ln.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	moved, err := moveConnOffStdFD(accepted)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	defer moved.Close()

	if fd := connFD(moved); fd <= 2 {
		t.Fatalf("moved connection still on std fd: %d", fd)
	}
	if err := <-dialErr; err != nil {
		t.Fatalf("dial: %v", err)
	}
}

// TestShutdownGracefulForcesStop 验证 GracefulStop 被在途请求阻塞时，
// 会在超时后强制 Stop，保证进程能退出。
func TestShutdownGracefulForcesStop(t *testing.T) {
	oldTimeout := shutdownTimeout
	shutdownTimeout = 200 * time.Millisecond
	defer func() { shutdownTimeout = oldTimeout }()

	started := make(chan struct{})
	s := grpc.NewServer(grpc.UnknownServiceHandler(
		func(_ any, stream grpc.ServerStream) error {
			close(started)
			<-stream.Context().Done()
			return stream.Context().Err()
		},
	))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = s.Serve(ln) }()

	conn, err := grpc.NewClient(ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer conn.Close()

	go func() {
		_ = conn.Invoke(context.Background(), "/test.Blocking/Call",
			&emptypb.Empty{}, &emptypb.Empty{})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking handler was not invoked")
	}

	start := time.Now()
	shutdownGraceful(s, "test")
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("graceful shutdown took too long: %s", elapsed)
	}
}
