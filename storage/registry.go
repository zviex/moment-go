package storage

import (
	"fmt"
	"sync"
)

// Options 是构造后端时的通用参数，各后端按需解释（如磁盘用 root，S3 用 bucket 等）。
type Options map[string]string

// Factory 根据参数构造一个存储后端。
type Factory func(opts Options) (Storage, error)

var (
	regMu     sync.RWMutex
	factories = map[string]Factory{}
)

func init() {
	Register(DiskKind, func(opts Options) (Storage, error) {
		return NewDisk(opts["root"]), nil
	})
}

// Register 注册一个后端工厂。重复注册同名后端会 panic。
func Register(kind string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()

	if _, dup := factories[kind]; dup {
		panic(fmt.Sprintf("storage: backend %q already registered", kind))
	}

	factories[kind] = f
}

// New 按 kind 构造存储后端；kind 为空时使用默认的磁盘后端。
func New(kind string, opts Options) (Storage, error) {
	if kind == "" {
		kind = DiskKind
	}

	regMu.RLock()

	f, ok := factories[kind]

	regMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("storage: unknown backend %q", kind)
	}

	return f(opts)
}

// Default 返回默认的磁盘存储（根目录 DefaultDiskRoot）。
func Default() Storage {
	return NewDisk("")
}
