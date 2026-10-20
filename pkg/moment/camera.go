package moment

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// maxWalkDepth 防止相机返回异常深度的目录树时无限递归。
const maxWalkDepth = 32

// Camera 表示一个已连接相机会话。它直接转发 Session 的能力，
// 不再额外维护一套领域层抽象。
type Camera struct {
	client  *Client
	session Session
}

func (c *Camera) Model() string { return c.session.Model() }

func (c *Camera) Driver() string { return c.session.Driver() }

func (c *Camera) PortPath() string { return c.session.PortPath() }

// ListFolders 列出指定目录下的子目录。根目录为 "/"。
func (c *Camera) ListFolders(ctx context.Context, folder string) ([]Folder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.session.ListFolders(ctx, folder)
}

// ListFiles 列出指定目录下的文件（含文件信息）。
func (c *Camera) ListFiles(ctx context.Context, folder string) ([]File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.session.ListFiles(ctx, folder)
}

// ListTree 递归遍历相机全部目录与文件，用于预览所有相机内容。
func (c *Camera) ListTree(ctx context.Context) (*CameraContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	folders := []Folder{{Name: "/", Path: "/"}}
	files := make([]File, 0)
	if err := walk(ctx, c.session, "/", 0, &folders, &files); err != nil {
		return nil, err
	}
	return &CameraContent{Folders: folders, Files: files}, nil
}

// walk 深度优先遍历目录树。
func walk(ctx context.Context, fs ReadOnlyFS, path string, depth int, folders *[]Folder, files *[]File) error {
	if depth > maxWalkDepth {
		return fmt.Errorf("directory tree too deep at %q", path)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	sub, err := fs.ListFolders(ctx, path)
	if err != nil {
		// 某些相机对根目录下的特殊存储节点不支持列目录，跳过并记录。
		slog.Warn("list folders failed", "folder", path, "err", err)
	} else {
		for _, f := range sub {
			*folders = append(*folders, f)
			if err := walk(ctx, fs, f.Path, depth+1, folders, files); err != nil {
				return err
			}
		}
	}

	entries, err := fs.ListFiles(ctx, path)
	if err != nil {
		return err
	}
	*files = append(*files, entries...)
	return nil
}

func (c *Camera) GetFileInfo(ctx context.Context, folder, name string) (File, error) {
	if err := ctx.Err(); err != nil {
		return File{}, err
	}
	return c.session.GetFileInfo(ctx, folder, name)
}

func (c *Camera) Open(ctx context.Context, folder, name string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.session.Open(ctx, folder, name)
}

func (c *Camera) Download(ctx context.Context, folder, name string) ([]byte, error) {
	r, err := c.Open(ctx, folder, name)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (c *Camera) DownloadTo(ctx context.Context, folder, name string, w io.Writer) error {
	r, err := c.Open(ctx, folder, name)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(w, r)
	return err
}

// GetInfo 聚合相机信息：摘要、驱动说明、存储、配置树。
func (c *Camera) GetInfo(ctx context.Context) (CameraInfo, error) {
	if err := ctx.Err(); err != nil {
		return CameraInfo{}, err
	}
	return c.session.GetInfo(ctx)
}

func (c *Camera) Close(ctx context.Context) error { return c.client.Disconnect(ctx) }
