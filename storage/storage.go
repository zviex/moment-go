package storage

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
)

type Storage interface {
	Path() string
}

type (
	Cache struct{}
	Disk  struct{}
)

func (d *Disk) Path() string {
	return ""
}

func (c *Cache) Path() string {
	return ""
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
	file, err := os.Create(filepath.Join(s.Path(), name))
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
