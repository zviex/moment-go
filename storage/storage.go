package storage

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type Storage interface {
	Path() string
}

type (
	Cache struct{}
	Disk  struct{}
)

// diskBase 是 Disk 存储的物理根目录。所有下载的图片文件都落盘在这里，绝不用 cache。
var (
	diskMu   sync.Mutex
	diskBase = filepath.Join("moment", "data")
)

// SetDiskBase 覆盖 Disk 存储的根目录（例如读取配置后调用，需先于首次写入）。
func SetDiskBase(dir string) {
	diskMu.Lock()
	defer diskMu.Unlock()
	diskBase = dir
}

// Path 返回 Disk 存储的根目录，首次调用时会惰性创建该目录。
func (d *Disk) Path() string {
	diskMu.Lock()
	dir := diskBase
	diskMu.Unlock()
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func (c *Cache) Path() string {
	return os.TempDir()
}

func Read(s Storage, name string) (io.Reader, error) {
	return read(s, name)
}

func Write(s Storage, name string, writeFunc func(io.Writer) error) error {
	return write(s, name, writeFunc)
}

func read(s Storage, name string) (io.Reader, error) {
	file, err := os.Open(filepath.Join(s.Path(), name))
	if err != nil {
		return nil, err
	}
	return bufio.NewReader(file), nil
}

func write(s Storage, name string, writeFunc func(io.Writer) error) error {
	dir := s.Path()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer file.Close()

	bufWriter := bufio.NewWriter(file)

	if err := writeFunc(bufWriter); err != nil {
		return err
	}

	return bufWriter.Flush()
}
