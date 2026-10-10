package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"time"
)

var ErrNotExist = fs.ErrNotExist

var ErrInvalidName = errors.New("storage: invalid file name")

type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// Storage 是通用文件存储接口。name 始终是相对于存储根的、使用 "/" 分隔的路径。
//
// 实现约定：
//   - Create 返回的 WriteCloser 在 Close 成功后才算提交；Close 前内容不保证可见。
//   - Create/Open 等需尊重 ctx 取消。
//   - 不存在的对象返回的错误需满足 errors.Is(err, ErrNotExist)。
type Storage interface {
	Create(ctx context.Context, name string) (io.WriteCloser, error)
	Open(ctx context.Context, name string) (io.ReadCloser, error)
	Stat(ctx context.Context, name string) (FileInfo, error)
	Remove(ctx context.Context, name string) error
}

// Read 打开 name 的内容，调用方负责 Close。
func Read(ctx context.Context, s Storage, name string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.Open(ctx, name)
}

// Write 通过 writeFunc 把内容写入 name。
// 写入过程可被 ctx 取消；失败时尽力删除未完成的对象。
func Write(ctx context.Context, s Storage, name string, writeFunc func(io.Writer) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	w, err := s.Create(ctx, name)
	if err != nil {
		return err
	}

	if err := writeFunc(ctxWriter{ctx: ctx, w: w}); err != nil {
		_ = w.Close()
		_ = s.Remove(context.WithoutCancel(ctx), name)

		return err
	}

	if err := ctx.Err(); err != nil {
		_ = w.Close()
		_ = s.Remove(context.WithoutCancel(ctx), name)

		return err
	}

	if err := w.Close(); err != nil {
		_ = s.Remove(context.WithoutCancel(ctx), name)
		return err
	}

	return nil
}

// ctxWriter 在每次写入前检查 ctx，使写入过程可被取消。
type ctxWriter struct {
	ctx context.Context
	w   io.Writer
}

func (cw ctxWriter) Write(p []byte) (int, error) {
	if err := cw.ctx.Err(); err != nil {
		return 0, err
	}

	return cw.w.Write(p)
}

// CleanName 校验并规范化对象名：去掉前导 "/"、统一分隔符，拒绝越出根目录的路径。
func CleanName(name string) (string, error) {
	n := filepath.ToSlash(name)
	for len(n) > 0 && n[0] == '/' {
		n = n[1:]
	}

	if n == "" {
		return "", ErrInvalidName
	}

	if !filepath.IsLocal(filepath.FromSlash(n)) {
		return "", ErrInvalidName
	}

	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(n))), nil
}
