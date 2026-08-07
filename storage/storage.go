package storage

import "io"

type Storage interface {
	Read(name string) (io.Reader, error)
	Write(name string, writeFunc func(io.Writer) error) error
	Path() string
}

func Load() {
}
