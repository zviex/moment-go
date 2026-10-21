package moment

import (
	"context"
	"io"

	"ci.zevix.uk/moment-go/adapter"
	"ci.zevix.uk/moment-go/adapter/ports"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

var (
	_ ports.FileBrowser   = (*FileAccess)(nil)
	_ ports.FileReader    = (*FileAccess)(nil)
	_ ports.CameraInfo    = (*InfoAccess)(nil)
	_ ports.CaptureSource = (*CaptureAccess)(nil)
)

// sessionProvider 是 Client 的能力代理所需的最小内部接口。
type sessionProvider interface {
	withSession(ctx context.Context, fn func(context.Context, adapter.Session) error) error
	borrowSession(ctx context.Context) (adapter.Session, context.Context, func(), error)
}

// FileAccess 是绑定当前会话的文件能力代理，不持有会话句柄本身。
type FileAccess struct {
	client sessionProvider
}

func NewFileAccess(client *Client) *FileAccess {
	return &FileAccess{client: client}
}

func (f *FileAccess) List(ctx context.Context, folder string) ([]*camerav1.FileContent, error) {
	var out []*camerav1.FileContent
	err := f.client.withSession(ctx, func(ctx context.Context, sess adapter.Session) error {
		var err error
		out, err = sess.List(ctx, folder)
		return err
	})
	return out, err
}

func (f *FileAccess) GetFileInfo(ctx context.Context, folder, name string) (*camerav1.FileEntry, error) {
	var out *camerav1.FileEntry
	err := f.client.withSession(ctx, func(ctx context.Context, sess adapter.Session) error {
		var err error
		out, err = sess.GetFileInfo(ctx, folder, name)
		return err
	})
	return out, err
}

func (f *FileAccess) DownloadTo(ctx context.Context, folder, name string, w io.Writer) error {
	return f.client.withSession(ctx, func(ctx context.Context, sess adapter.Session) error {
		return sess.DownloadTo(ctx, folder, name, w)
	})
}

// InfoAccess 是绑定当前会话的相机信息代理。
type InfoAccess struct {
	client sessionProvider
}

func NewInfoAccess(client *Client) *InfoAccess {
	return &InfoAccess{client: client}
}

func (i *InfoAccess) GetInfo(ctx context.Context) (*camerav1.CameraSummary, error) {
	var out *camerav1.CameraSummary
	err := i.client.withSession(ctx, func(ctx context.Context, sess adapter.Session) error {
		var err error
		out, err = sess.GetInfo(ctx)
		return err
	})
	return out, err
}

// CaptureAccess 是绑定当前会话的拍摄事件代理。
// 事件流会持有一个会话租约，连接断开时租约随事件流一起释放。
type CaptureAccess struct {
	client sessionProvider
}

func NewCaptureAccess(client *Client) *CaptureAccess {
	return &CaptureAccess{client: client}
}

func (c *CaptureAccess) Events(ctx context.Context) (<-chan ports.CaptureEvent, error) {
	sess, sessionCtx, release, err := c.client.borrowSession(ctx)
	if err != nil {
		return nil, err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(sessionCtx, cancel)

	src, err := sess.Events(streamCtx)
	if err != nil {
		stop()
		cancel()
		release()
		return nil, err
	}

	out := make(chan ports.CaptureEvent)
	go func() {
		defer close(out)
		defer release()
		defer stop()
		defer cancel()

		for {
			select {
			case <-streamCtx.Done():
				return
			case ev, ok := <-src:
				if !ok {
					return
				}
				select {
				case out <- ev:
				case <-streamCtx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}
