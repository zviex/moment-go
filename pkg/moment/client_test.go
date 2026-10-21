package moment

import (
	"bytes"
	"context"
	"io"
	"testing"

	"ci.zevix.uk/moment-go/adapter"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

// fakeBackend 提供标准化 Connect 与 Discoverer，用于无硬件测试。
type fakeBackend struct{}

func (fakeBackend) Detect(context.Context) ([]adapter.DetectedCamera, error) {
	return []adapter.DetectedCamera{{Model: "TestCam", PortPath: "usb:001,002"}}, nil
}

func (fakeBackend) Connect(ctx context.Context, _ *camerav1.Tartget) (adapter.Session, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &fakeSession{tree: fakeTree(), files: fakeFiles()}, nil
}

type fakeSession struct {
	tree  map[string][]string
	files map[string][]*camerav1.FileEntry
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

func (s *fakeSession) ListFiles(_ context.Context, folder string) ([]*camerav1.FileEntry, error) {
	if _, ok := s.tree[folder]; !ok {
		if _, ok := s.files[folder]; !ok {
			return nil, ErrDirectoryNotFound
		}
	}
	out := make([]*camerav1.FileEntry, 0, len(s.files[folder]))
	for _, f := range s.files[folder] {
		f.Floder = folder
		out = append(out, f)
	}
	return out, nil
}

func (s *fakeSession) GetFileInfo(_ context.Context, folder, name string) (*camerav1.FileEntry, error) {
	for _, f := range s.files[folder] {
		if f.Name == name {
			return f, nil
		}
	}
	return nil, ErrFileNotFound
}

func (s *fakeSession) Open(_ context.Context, folder, name string) (io.ReadCloser, error) {
	f, err := s.GetFileInfo(context.Background(), folder, name)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), int(f.Size)))), nil
}

func (s *fakeSession) GetInfo(context.Context) (*camerav1.CameraSummary, error) {
	return &camerav1.CameraSummary{Model: "TestCam", Driver: "fake", PortPath: "usb:001,002"}, nil
}

func (s *fakeSession) Close(context.Context) error { return nil }

func (s *fakeSession) List(ctx context.Context, folder string) ([]*camerav1.FileContent, error) {
	return nil, nil
}

func newTestClient() *Client {
	b := fakeBackend{}
	return NewClient(b.Connect, b)
}

func TestConnectLifecycle(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()

	cam, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if cam == nil || !c.IsConnected() {
		t.Fatal("expected connected camera")
	}

	same, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"})
	if err != nil {
		t.Fatalf("second connect should reuse session: %v", err)
	}
	if same.session != cam.session {
		t.Fatal("second connect should return the same connected session")
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
	c := newTestClient()

	if _, err := c.Camera(); err != ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestDownloadRoundTrip(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()
	cam, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"})
	if err != nil {
		t.Fatal(err)
	}
	defer cam.Close(ctx)

	data, err := cam.Download(ctx, "/store_00010001/DCIM/100CANON", "IMG_0001.JPG")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if len(data) != 2048 {
		t.Fatalf("downloaded %d bytes, want 2048", len(data))
	}

	var buf bytes.Buffer
	if err := cam.DownloadTo(ctx, "/store_00010001/DCIM/100CANON", "IMG_0002.JPG", &buf); err != nil {
		t.Fatalf("downloadTo: %v", err)
	}
	if buf.Len() != 4096 {
		t.Fatalf("downloadTo %d bytes, want 4096", buf.Len())
	}
}

func TestGetInfo(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()
	cam, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"})
	if err != nil {
		t.Fatal(err)
	}
	defer cam.Close(ctx)

	info, err := cam.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Model != "TestCam" || info.Driver != "fake" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestConnectCanceledContext(t *testing.T) {
	c := newTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Connect(ctx, &camerav1.Tartget{PortPath: "usb:001,002", Model: "TestCam"})
	if err == nil {
		t.Fatal("expected canceled context error")
	}
}
