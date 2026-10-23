package moment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"ci.zevix.uk/moment-go/adapter"
	"ci.zevix.uk/moment-go/adapter/ports"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

// fakeBackend 提供标准化 Connect 与 Discoverer，用于无硬件测试。
type fakeBackend struct {
	newSession func() adapter.Session
}

func (b fakeBackend) Detect(context.Context) ([]adapter.DetectedCamera, error) {
	return []adapter.DetectedCamera{{Model: "TestCam", PortPath: "usb:001,002"}}, nil
}

func (b fakeBackend) Connect(ctx context.Context, _ *camerav1.Tartget) (adapter.Session, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if b.newSession != nil {
		return b.newSession(), nil
	}
	return newFakeSession(), nil
}

type fakeSession struct {
	tree  map[string][]string
	files map[string][]*camerav1.FileEntry

	blockDownload bool
	started       chan struct{}
	startOnce     sync.Once
}

func newFakeSession() *fakeSession {
	return &fakeSession{tree: fakeTree(), files: fakeFiles()}
}

func fakeTree() map[string][]string {
	return map[string][]string{
		"/":                             {"store_00010001"},
		"/store_00010001":               {"DCIM"},
		"/store_00010001/DCIM":          {"100CANON"},
		"/store_00010001/DCIM/100CANON": {},
	}
}

func fakeFiles() map[string][]*camerav1.FileEntry {
	return map[string][]*camerav1.FileEntry{
		"/store_00010001/DCIM/100CANON": {
			{Name: "IMG_0001.JPG", Size: 2048, MimeType: "image/jpeg"},
			{Name: "IMG_0002.JPG", Size: 4096, MimeType: "image/jpeg"},
		},
	}
}

func (s *fakeSession) Model() string    { return "TestCam" }
func (s *fakeSession) Driver() string   { return "fake" }
func (s *fakeSession) PortPath() string { return "usb:001,002" }

func (s *fakeSession) List(_ context.Context, folder string) (*camerav1.FileContent, error) {
	if _, ok := s.tree[folder]; !ok {
		if _, ok := s.files[folder]; !ok {
			return nil, ErrDirectoryNotFound
		}
	}
	return &camerav1.FileContent{
		Folders: s.tree[folder],
		Files:   s.files[folder],
	}, nil
}

func (s *fakeSession) GetFileInfo(_ context.Context, folder, name string) (*camerav1.FileEntry, error) {
	for _, f := range s.files[folder] {
		if f.Name == name {
			return f, nil
		}
	}
	return nil, ErrFileNotFound
}

func (s *fakeSession) DownloadTo(ctx context.Context, folder, name string, w io.Writer) error {
	if s.blockDownload {
		s.startOnce.Do(func() {
			if s.started != nil {
				close(s.started)
			}
		})
		<-ctx.Done()
		return ctx.Err()
	}

	f, err := s.GetFileInfo(ctx, folder, name)
	if err != nil {
		return err
	}
	_, err = w.Write(bytes.Repeat([]byte("x"), int(f.Size)))
	return err
}

func (s *fakeSession) GetInfo(context.Context) (*camerav1.CameraSummary, error) {
	return &camerav1.CameraSummary{Model: "TestCam", Driver: "fake", PortPath: "usb:001,002"}, nil
}

func (s *fakeSession) Events(ctx context.Context) (<-chan ports.CaptureEvent, error) {
	ch := make(chan ports.CaptureEvent)
	go func() {
		defer close(ch)
		<-ctx.Done()
	}()
	return ch, nil
}

func (s *fakeSession) Close(context.Context) error { return nil }

func newTestClient() (*Client, *fakeBackend) {
	b := &fakeBackend{}
	return NewClient(b.Connect, b), b
}

func TestConnectLifecycle(t *testing.T) {
	c, _ := newTestClient()
	ctx := context.Background()

	info, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if info.Model != "TestCam" || !c.IsConnected() {
		t.Fatalf("unexpected connection info: %+v, connected=%v", info, c.IsConnected())
	}

	same, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"})
	if err != nil {
		t.Fatalf("second connect should reuse session: %v", err)
	}
	if same != info {
		t.Fatalf("second connect info = %+v, want %+v", same, info)
	}

	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if c.IsConnected() {
		t.Fatal("expected disconnected")
	}
	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect again should be no-op: %v", err)
	}
}

func TestOperationsRequireConnection(t *testing.T) {
	c, _ := newTestClient()

	if _, err := NewFileAccess(c).List(context.Background(), "/"); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
	if _, err := NewInfoAccess(c).GetInfo(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestDownloadRoundTrip(t *testing.T) {
	c, _ := newTestClient()
	ctx := context.Background()
	if _, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	files := NewFileAccess(c)
	var buf bytes.Buffer
	if err := files.DownloadTo(ctx, "/store_00010001/DCIM/100CANON", "IMG_0001.JPG", &buf); err != nil {
		t.Fatalf("download: %v", err)
	}
	if buf.Len() != 2048 {
		t.Fatalf("downloaded %d bytes, want 2048", buf.Len())
	}

	info, err := files.GetFileInfo(ctx, "/store_00010001/DCIM/100CANON", "IMG_0002.JPG")
	if err != nil {
		t.Fatalf("get file info: %v", err)
	}
	if info.GetSize() != 4096 {
		t.Fatalf("file size = %d, want 4096", info.GetSize())
	}
}

func TestGetInfo(t *testing.T) {
	c, _ := newTestClient()
	ctx := context.Background()
	if _, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	info, err := NewInfoAccess(c).GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Model != "TestCam" || info.Driver != "fake" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestDisconnectCancelsInFlightOperation(t *testing.T) {
	started := make(chan struct{})
	c, _ := newTestClient()
	c.connect = func(ctx context.Context, _ *camerav1.Tartget) (adapter.Session, error) {
		s := newFakeSession()
		s.blockDownload = true
		s.started = started
		return s, nil
	}

	ctx := context.Background()
	if _, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"}); err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- NewFileAccess(c).DownloadTo(ctx, "/store_00010001/DCIM/100CANON", "IMG_0001.JPG", io.Discard)
	}()

	<-started
	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight download error = %v, want context.Canceled", err)
		}
	default:
		// Disconnect 会等待租约释放，因此这里理论上已经结束。
		t.Fatal("in-flight download did not finish before disconnect returned")
	}
}

func TestCaptureStreamClosesOnDisconnect(t *testing.T) {
	c, _ := newTestClient()
	ctx := context.Background()
	if _, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"}); err != nil {
		t.Fatal(err)
	}

	ch, err := NewCaptureAccess(c).Events(ctx)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected capture stream to be closed")
		}
	default:
		t.Fatal("capture stream did not close on disconnect")
	}
}

func TestConnectCanceledContext(t *testing.T) {
	c, _ := newTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"})
	if err == nil {
		t.Fatal("expected canceled context error")
	}
}
