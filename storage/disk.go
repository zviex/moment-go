package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

const DiskKind = "disk"

// DefaultDataDir 返回默认数据目录（相对于工作目录）。
func DefaultDataDir() string { return filepath.Join("moment", "data") }

type Disk struct {
	root string
}

func NewDisk(root string) *Disk {
	if root == "" {
		root = DefaultDataDir()
	}

	return &Disk{root: root}
}

func (d *Disk) Root() string { return d.root }

func (d *Disk) Create(ctx context.Context, name string) (io.WriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p, err := d.path(name)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}

	return os.Create(p)
}

func (d *Disk) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p, err := d.path(name)
	if err != nil {
		return nil, err
	}

	return os.Open(p)
}

func (d *Disk) Stat(ctx context.Context, name string) (FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return FileInfo{}, err
	}

	p, err := d.path(name)
	if err != nil {
		return FileInfo{}, err
	}

	fi, err := os.Stat(p)
	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{Name: name, Size: fi.Size(), ModTime: fi.ModTime()}, nil
}

func (d *Disk) Remove(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p, err := d.path(name)
	if err != nil {
		return err
	}

	return os.Remove(p)
}

func (d *Disk) path(name string) (string, error) {
	n, err := CleanName(name)
	if err != nil {
		return "", err
	}

	return filepath.Join(d.root, filepath.FromSlash(n)), nil
}
