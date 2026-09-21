package gphoto2

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBootstrapResolveDriverDirs 校验 Bootstrap 能从布局根目录自动探测带版本号的
// camlib / iolib 目录，并写入 CAMLIBS / IOLIBS。
func TestBootstrapResolveDriverDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "libgphoto2", "2.5.34.1", "ptp2.so"))
	writeFile(t, filepath.Join(root, "libgphoto2_port", "0.12.2", "ptpip.so"))

	t.Setenv("CAMLIBS", "keep")
	t.Setenv("IOLIBS", "keep")

	if err := Bootstrap(root, "", ""); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	if got, want := os.Getenv("CAMLIBS"), filepath.Join(root, "libgphoto2", "2.5.34.1"); got != want {
		t.Fatalf("CAMLIBS = %q, want %q", got, want)
	}
	if got, want := os.Getenv("IOLIBS"), filepath.Join(root, "libgphoto2_port", "0.12.2"); got != want {
		t.Fatalf("IOLIBS = %q, want %q", got, want)
	}
}

// TestBootstrapKeepsEnvWhenUnset 校验不传任何目录时保持系统默认（不改环境变量）。
func TestBootstrapKeepsEnvWhenUnset(t *testing.T) {
	t.Setenv("CAMLIBS", "keep")
	t.Setenv("IOLIBS", "keep")

	if err := Bootstrap("", "", ""); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if got := os.Getenv("CAMLIBS"); got != "keep" {
		t.Fatalf("CAMLIBS = %q, want unchanged", got)
	}
	if got := os.Getenv("IOLIBS"); got != "keep" {
		t.Fatalf("IOLIBS = %q, want unchanged", got)
	}
}

// TestBootstrapMissingLayout 校验布局根目录无效时返回明确错误。
func TestBootstrapMissingLayout(t *testing.T) {
	if err := Bootstrap(filepath.Join(t.TempDir(), "missing"), "", ""); err == nil {
		t.Fatal("Bootstrap with missing layout should fail")
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
