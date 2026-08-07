package disk

import "io"

type Disk struct{}

func (c *Disk) Read(name string) (io.Reader, error) {
	panic("unimplemented")
}

func Write(name string, writeFunc func(io.Writer) error) error {
	panic("unimplemented")
}

func (c Disk) Path() string {
	return ""
}
