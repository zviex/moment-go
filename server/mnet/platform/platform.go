package platform

import (
	"runtime"
)

var fd = fact{
	m: map[string]string{},
}

type fact struct {
	m map[string]string
}

func init() {
	fd.register(Linux{})
	fd.register(Android{})
	fd.register(MacOS{})
}

func (f fact) register(s SocketAddr) {
	f.m[s.OS()] = s.Addr()
}

type SocketAddr interface {
	OS() string
	Addr() string
}

func PlatformAddr() string {
	if str, ok := fd.m[runtime.GOOS]; ok {
		return str
	}

	panic("unsupport platform")
}
