package mnet

import (
	"errors"
	"io/fs"
	"os"

	"ci.zevix.uk/moment-go/server/mnet/platform"
)

type Net interface {
	Network() string
	Addr() string
	Cleanup() error
}

var _default Net

func DefaultNet() Net {
	return _default
}

func SetDefaultNet(n Net) {
	_default = n
}

func Init() {
	SetDefaultNet(NewUnix())
}

type Non struct{}

func (Non) Network() string {
	return "non"
}

func (Non) Addr() string {
	return "non"
}

func (Non) Cleanup() error {
	return nil
}

type Unix struct {
	addr string
}

func NewUnix() *Unix {
	return NewUnixAt(platform.PlatformAddr())
}

// NewUnixAt 使用显式地址创建监听配置，便于测试或自定义部署。
func NewUnixAt(addr string) *Unix {
	return &Unix{addr: addr}
}

func (u Unix) Network() string {
	return "unix"
}

func (u Unix) Addr() string {
	return u.addr
}

func (u Unix) Cleanup() error {
	if err := os.Remove(u.addr); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}
