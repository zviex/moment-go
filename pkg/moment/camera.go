package moment

import (
	"context"
	"io"

	"ci.zevix.uk/moment-go/adapter"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

// maxWalkDepth 防止相机返回异常深度的目录树时无限递归。
const maxWalkDepth = 32

// Camera 表示一个已连接相机会话。它直接转发 Session 的能力，
// 不再额外维护一套领域层抽象。
type Camera struct {
	client  *Client
	session adapter.Session
}

func (c *Camera) Model() string { return c.session.Model() }

func (c *Camera) Driver() string { return c.session.Driver() }

func (c *Camera) PortPath() string { return c.session.PortPath() }

// ListFiles 列出指定目录下的文件（含文件信息）。
func (c *Camera) ListFiles(ctx context.Context, folder string) ([]*camerav1.FileContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return c.session.List(ctx, folder)
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
func (c *Camera) GetInfo(ctx context.Context) (*camerav1.CameraSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.session.GetInfo(ctx)
}

func (c *Camera) Close(ctx context.Context) error { return c.client.Disconnect(ctx) }
