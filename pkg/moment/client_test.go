package moment

import (
	"bytes"
	"context"
	"io"
	"testing"
)

// fakeBackend 提供标准化 Connect 与 Discoverer，用于无硬件测试。
type fakeBackend struct{}

func (fakeBackend) Detect(context.Context) ([]DetectedCamera, error) {
	return []DetectedCamera{{Model: "TestCam", PortPath: "usb:001,002"}}, nil
}

func (fakeBackend) Connect(ctx context.Context, _ Target) (Session, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &fakeSession{tree: fakeTree(), files: fakeFiles()}, nil
}

type fakeSession struct {
	tree  map[string][]string
	files map[string][]File
}

func fakeTree() map[string][]string {
	return map[string][]string{
		"/":                             {"store_00010001"},
		"/store_00010001":               {"DCIM"},
		"/store_00010001/DCIM":          {"100CANON"},
		"/store_00010001/DCIM/100CANON": {},
	}
}

func fakeFiles() map[string][]File {
	return map[string][]File{
		"/store_00010001/DCIM/100CANON": {
			{Name: "IMG_0001.JPG", Size: 2048, MimeType: "image/jpeg"},
			{Name: "IMG_0002.JPG", Size: 4096, MimeType: "image/jpeg"},
		},
	}
}

func (s *fakeSession) Model() string    { return "TestCam" }
func (s *fakeSession) Driver() string   { return "fake" }
func (s *fakeSession) PortPath() string { return "usb:001,002" }

func (s *fakeSession) ListFolders(_ context.Context, folder string) ([]Folder, error) {
	names, ok := s.tree[folder]
	if !ok {
		return nil, ErrDirectoryNotFound
	}
	out := make([]Folder, 0, len(names))
	for _, n := range names {
		p := "/" + n
		if folder != "/" {
			p = folder + "/" + n
		}
		out = append(out, Folder{Name: n, Path: p})
	}
	return out, nil
}

func (s *fakeSession) ListFiles(_ context.Context, folder string) ([]File, error) {
	if _, ok := s.tree[folder]; !ok {
		if _, ok := s.files[folder]; !ok {
			return nil, ErrDirectoryNotFound
		}
	}
	out := make([]File, 0, len(s.files[folder]))
	for _, f := range s.files[folder] {
		f.Folder = folder
		out = append(out, f)
	}
	return out, nil
}

func (s *fakeSession) GetFileInfo(_ context.Context, folder, name string) (File, error) {
	for _, f := range s.files[folder] {
		if f.Name == name {
			return f, nil
		}
	}
	return File{}, ErrFileNotFound
}

func (s *fakeSession) Open(_ context.Context, folder, name string) (io.ReadCloser, error) {
	f, err := s.GetFileInfo(context.Background(), folder, name)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), int(f.Size)))), nil
}

func (s *fakeSession) GetInfo(context.Context) (CameraInfo, error) {
	return CameraInfo{Model: "TestCam", Driver: "fake", PortPath: "usb:001,002"}, nil
}

func (s *fakeSession) Close(context.Context) error { return nil }

func newTestClient() *Client {
	b := fakeBackend{}
	return NewClient(b.Connect, b)
}

func TestConnectLifecycle(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()

	cam, err := c.Connect(ctx, Target{PortPath: "usb:001,002", Model: "TestCam"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if cam == nil || !c.IsConnected() {
		t.Fatal("expected connected camera")
	}

	same, err := c.Connect(ctx, Target{PortPath: "usb:001,002", Model: "TestCam"})
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

func TestListTreeWalksAllFolders(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()
	cam, err := c.Connect(ctx, Target{PortPath: "usb:001,002", Model: "TestCam"})
	if err != nil {
		t.Fatal(err)
	}
	defer cam.Close(ctx)

	content, err := cam.ListTree(ctx)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(content.Folders) != 4 {
		t.Fatalf("folders = %d, want 4", len(content.Folders))
	}
	if len(content.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(content.Files))
	}
}

func TestDownloadRoundTrip(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()
	cam, err := c.Connect(ctx, Target{PortPath: "usb:001,002", Model: "TestCam"})
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
	cam, err := c.Connect(ctx, Target{PortPath: "usb:001,002", Model: "TestCam"})
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
	_, err := c.Connect(ctx, Target{PortPath: "usb:001,002", Model: "TestCam"})
	if err == nil {
		t.Fatal("expected canceled context error")
	}
}
