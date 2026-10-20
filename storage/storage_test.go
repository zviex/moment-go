package storage

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestDiskWriteReadRemove(t *testing.T) {
	ctx := context.Background()
	s := NewDisk(t.TempDir())

	if err := Write(ctx, s, "a/b.txt", func(w io.Writer) error {
		_, err := io.WriteString(w, "hello")
		return err
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, err := Read(ctx, s, "a/b.txt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "hello" {
		t.Fatalf("got %q", b)
	}

	fi, err := s.Stat(ctx, "a/b.txt")
	if err != nil || fi.Size != 5 {
		t.Fatalf("stat: %+v %v", fi, err)
	}

	if err := s.Remove(ctx, "a/b.txt"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := s.Stat(ctx, "a/b.txt"); !errors.Is(err, ErrNotExist) {
		t.Fatalf("want ErrNotExist, got %v", err)
	}
}

func TestDiskRejectsEscapingNames(t *testing.T) {
	s := NewDisk(t.TempDir())
	for _, name := range []string{"../x", "a/../../x", ""} {
		if _, err := s.Open(context.Background(), name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("name %q: want ErrInvalidName, got %v", name, err)
		}
	}
}

func TestWriteFailureRemovesObject(t *testing.T) {
	ctx := context.Background()
	s := NewDisk(t.TempDir())
	wantErr := errors.New("boom")
	err := Write(ctx, s, "partial.bin", func(w io.Writer) error {
		_, _ = w.Write([]byte("part"))
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("want boom, got %v", err)
	}
	if _, err := s.Stat(ctx, "partial.bin"); !errors.Is(err, ErrNotExist) {
		t.Fatalf("partial object should be removed, got %v", err)
	}
}
