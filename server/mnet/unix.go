package mnet

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"ci.zevix.uk/moment-go/server/mnet/platform"
)

type Net interface {
	Network() string
	Addr() string
	Cleanup() error
}

type Non struct{}

func (Non) Network() string { return "non" }
func (Non) Addr() string    { return "non" }
func (Non) Cleanup() error  { return nil }

type Unix struct {
	addr string
}

func NewUnix() *Unix {
	return NewUnixAt(platform.PlatformAddr())
}

// NewUnixAt 使用显式地址创建监听配置，便于测试或自定义部署。
func NewUnixAt(addr string) *Unix {
	slog.Debug("unix socket configured", "network", "unix", "addr", addr)
	return &Unix{addr: addr}
}

func (u Unix) Network() string { return "unix" }
func (u Unix) Addr() string    { return u.addr }

func (u Unix) Cleanup() error {
	slog.Debug("unix socket cleanup", "addr", u.addr)
	if err := os.Remove(u.addr); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Debug("unix socket cleanup failed", "addr", u.addr, "err", err)
		return err
	}
	return nil
}
