package cache

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
)

type Cache struct{}

func (c *Cache) Read(name string) (io.Reader, error) {
	file, err := os.Open(filepath.Join(c.Path(), name))
	if err != nil {
		return nil, err
	}
	return bufio.NewReader(file), nil
}

func (c *Cache) Write(name string, writeFunc func(io.Writer) error) error {
	file, err := os.Create(filepath.Join(c.Path(), name))
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

func (c Cache) Path() string {
	return ""
}
