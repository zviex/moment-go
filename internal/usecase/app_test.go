package usecase

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"ci.zevix.uk/moment-go/internal/domain"
	"ci.zevix.uk/moment-go/internal/logging"
)

// ---- 内存假后端，用于在无硬件环境验证用例编排 ----

type fakeGateway struct{}

func (fakeGateway) Detect(context.Context) ([]domain.DetectedCamera, error) {
	return []domain.DetectedCamera{{Model: "TestCam", PortPath: "usb:001,002"}}, nil
}

func (fakeGateway) Connect(ctx context.Context, _ domain.ConnectOptions) (domain.CameraSession, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &fakeSession{tree: fakeTree(), files: fakeFiles()}, nil
}

func (fakeGateway) Close() error { return nil }

type fakeSession struct {
	tree  map[string][]string // folder -> subfolder names
	files map[string][]domain.File
}

func fakeTree() map[string][]string {
	return map[string][]string{
		"/":                             {"store_00010001"},
		"/store_00010001":               {"DCIM"},
		"/store_00010001/DCIM":          {"100CANON"},
		"/store_00010001/DCIM/100CANON": {},
	}
}

func fakeFiles() map[string][]domain.File {
	return map[string][]domain.File{
		"/store_00010001/DCIM/100CANON": {
			{Name: "IMG_0001.JPG", Size: 2048, MimeType: "image/jpeg"},
			{Name: "IMG_0002.JPG", Size: 4096, MimeType: "image/jpeg"},
		},
	}
}

func (s *fakeSession) Model() string    { return "TestCam" }
func (s *fakeSession) Driver() string   { return "fake" }
func (s *fakeSession) PortPath() string { return "usb:001,002" }

func (s *fakeSession) ListFolders(_ context.Context, folder string) ([]domain.Folder, error) {
	names, ok := s.tree[folder]
	if !ok {
		return nil, domain.ErrDirectoryNotFound
	}
	out := make([]domain.Folder, 0, len(names))
	for _, n := range names {
		p := n
		if folder != "/" {
			p = folder + "/" + n
		} else {
			p = "/" + n
		}
		out = append(out, domain.Folder{Name: n, Path: p})
	}
	return out, nil
}

func (s *fakeSession) ListFiles(_ context.Context, folder string) ([]domain.File, error) {
	if _, ok := s.tree[folder]; !ok {
		if _, ok := s.files[folder]; !ok {
			return nil, domain.ErrDirectoryNotFound
		}
	}
	out := make([]domain.File, 0, len(s.files[folder]))
	for _, f := range s.files[folder] {
		f.Folder = folder
		out = append(out, f)
	}
	return out, nil
}

func (s *fakeSession) GetFileInfo(_ context.Context, folder, name string) (domain.File, error) {
	for _, f := range s.files[folder] {
		if f.Name == name {
			return f, nil
		}
	}
	return domain.File{}, domain.ErrFileNotFound
}

func (s *fakeSession) Download(_ context.Context, folder, name string) ([]byte, error) {
	f, err := s.GetFileInfo(context.Background(), folder, name)
	if err != nil {
		return nil, err
	}
	return bytes.Repeat([]byte("x"), int(f.Size)), nil
}

func (s *fakeSession) DownloadTo(ctx context.Context, folder, name string, w io.Writer) error {
	data, err := s.Download(ctx, folder, name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (s *fakeSession) GetInfo(_ context.Context) (domain.CameraInfo, error) {
	return domain.CameraInfo{Model: "TestCam", Driver: "fake", PortPath: "usb:001,002"}, nil
}

func (s *fakeSession) Close(context.Context) error { return nil }

// ---- 用例测试 ----

func newTestApp() *CameraApp {
	return NewCameraApp(fakeGateway{}, logging.Nop{})
}

func TestConnectLifecycle(t *testing.T) {
	app := newTestApp()
	ctx := context.Background()

	sess, err := app.Connect(ctx, domain.ConnectOptions{Type: domain.ConnectAuto})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if sess == nil {
		t.Fatal("nil session")
	}
	if !app.IsConnected() {
		t.Fatal("expected connected")
	}

	// 重复连接应报错。
	if _, err := app.Connect(ctx, domain.ConnectOptions{Type: domain.ConnectAuto}); err == nil {
		t.Fatal("second connect should fail")
	}

	if err := app.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if app.IsConnected() {
		t.Fatal("expected disconnected")
	}
	// 再次断开应幂等。
	if err := app.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect again should be no-op: %v", err)
	}
}

func TestOperationsRequireConnection(t *testing.T) {
	app := newTestApp()
	ctx := context.Background()

	if _, err := app.ListFiles(ctx, "/"); err != domain.ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
	if _, _, err := app.ListTree(ctx); err != domain.ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
	if _, err := app.Download(ctx, "/", "x.jpg"); err != domain.ErrNotConnected {
		t.Fatalf("expected ErrNotConnected, got %v", err)
	}
}

func TestListTreeWalksAllFolders(t *testing.T) {
	app := newTestApp()
	ctx := context.Background()
	if _, err := app.Connect(ctx, domain.ConnectOptions{Type: domain.ConnectAuto}); err != nil {
		t.Fatal(err)
	}
	defer app.Disconnect(ctx)

	folders, files, err := app.ListTree(ctx)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	// 根 + store_00010001 + DCIM + 100CANON
	if len(folders) != 4 {
		t.Fatalf("folders = %d, want 4", len(folders))
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	for _, f := range files {
		if f.Size != 2048 && f.Size != 4096 {
			t.Fatalf("unexpected file size %d", f.Size)
		}
	}
}

func TestDownloadRoundTrip(t *testing.T) {
	app := newTestApp()
	ctx := context.Background()
	if _, err := app.Connect(ctx, domain.ConnectOptions{Type: domain.ConnectAuto}); err != nil {
		t.Fatal(err)
	}
	defer app.Disconnect(ctx)

	data, err := app.Download(ctx, "/store_00010001/DCIM/100CANON", "IMG_0001.JPG")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if len(data) != 2048 {
		t.Fatalf("downloaded %d bytes, want 2048", len(data))
	}

	var buf bytes.Buffer
	if err := app.DownloadTo(ctx, "/store_00010001/DCIM/100CANON", "IMG_0002.JPG", &buf); err != nil {
		t.Fatalf("downloadTo: %v", err)
	}
	if buf.Len() != 4096 {
		t.Fatalf("downloadTo %d bytes, want 4096", buf.Len())
	}
}

func TestGetInfo(t *testing.T) {
	app := newTestApp()
	ctx := context.Background()
	if _, err := app.Connect(ctx, domain.ConnectOptions{Type: domain.ConnectAuto}); err != nil {
		t.Fatal(err)
	}
	defer app.Disconnect(ctx)

	info, err := app.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Model != "TestCam" || info.Driver != "fake" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestConnectTimeout(t *testing.T) {
	app := newTestApp()
	ctx := context.Background()
	// 超时极短，验证超时包装路径可用。
	opts := domain.ConnectOptions{Type: domain.ConnectAuto, Timeout: time.Nanosecond}
	_, err := app.Connect(ctx, opts)
	if err == nil {
		t.Fatal("expected timeout/cancel error")
	}
}
