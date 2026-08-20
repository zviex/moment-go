package mnet

import (
	"ci.zevix.uk/moment-go/config"
)

type Net interface {
	Network() string
	Addr() string
}

var _default = NewUnix()

func DefaultNet() Net {
	return _default
}

type Unix struct{}

func NewUnix() *Unix {
	u := &Unix{}
	return u
}

func (u Unix) Network() string {
	return "unix"
}

func (u Unix) Addr() string {
	return config.Default().NetPath
}
