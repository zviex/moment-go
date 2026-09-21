package moment

import (
	"context"
	"io"

	"ci.zevix.uk/moment-go/internal/adapter/gphoto2"
	"ci.zevix.uk/moment-go/internal/domain"
	"ci.zevix.uk/moment-go/internal/usecase"
)

// Client 是相机会话的入口，管理生命周期并执行用例。
type Client struct {
	app *usecase.CameraApp
}

// New 创建 Client。可通过 Option 配置日志与 libgphoto2 运行库目录。
func New(opts ...Option) (*Client, error) {
	o := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	// 按需设置 camlib/iolib 环境变量：桌面端留空即使用系统默认路径，
	// Android 需要指向随 APK 打包并解压出来的驱动目录。
	if err := gphoto2.Bootstrap(o.libraryRoot, o.camlibDir, o.iolibDir); err != nil {
		return nil, err
	}
	gw, err := gphoto2.NewGateway(o.logger)
	if err != nil {
		return nil, err
	}
	return &Client{app: usecase.NewCameraApp(gw, o.logger)}, nil
}

// State 返回当前生命周期状态。
func (c *Client) State() State { return State(c.app.State()) }

// IsConnected 是否已连接。
func (c *Client) IsConnected() bool { return c.app.IsConnected() }

// LastError 返回最近一次失败原因。
func (c *Client) LastError() error { return c.app.LastError() }

// Detect 探测可用相机（USB 自动检测）。
func (c *Client) Detect(ctx context.Context) ([]DetectedCamera, error) {
	cams, err := c.app.Detect(ctx)
	if err != nil {
		return nil, err
	}
	return toDetected(cams), nil
}

// Camera 返回当前已连接会话对应的 Camera 实例；如果当前未连接则返回 ErrNotConnected。
func (c *Client) Camera() (*Camera, error) {
	sess, err := c.app.Session()
	if err != nil {
		return nil, err
	}
	return &Camera{app: c.app, sess: sess}, nil
}

// Connect 建立相机连接，成功时返回 *Camera（生命周期进入 Connected）。
// 若客户端已连接，则直接返回当前 Camera 实例，不重复建立新连接。
//
//	// PTP/IP 网络连接：
//	cam, err := client.Connect(ctx, moment.ConnectOptions{
//		Type: moment.ConnectPTPIP,
//		Host: "192.168.1.100",
//	})
//
//	// USB 自动探测：
//	cam, err := client.Connect(ctx, moment.ConnectOptions{Type: moment.ConnectAuto})
func (c *Client) Connect(ctx context.Context, opts ConnectOptions) (*Camera, error) {
	if c.IsConnected() {
		return c.Camera()
	}
	sess, err := c.app.Connect(ctx, toDomainOptions(opts))
	if err != nil {
		return nil, err
	}
	return &Camera{app: c.app, sess: sess}, nil
}

// Disconnect 断开连接（生命周期回到 Disconnected）。
func (c *Client) Disconnect(ctx context.Context) error { return c.app.Disconnect(ctx) }

// Close 断开连接并释放网关底层资源。
func (c *Client) Close() error { return c.app.Close() }

// Camera 表示一个已连接相机会话，围绕生命周期提供浏览 / 下载 / 解析能力。
type Camera struct {
	app  *usecase.CameraApp
	sess domain.CameraSession
}

// Model 返回相机模型名。
func (c *Camera) Model() string { return c.sess.Model() }

// Driver 返回驱动（camlib）名。
func (c *Camera) Driver() string { return c.sess.Driver() }

// PortPath 返回端口路径。
func (c *Camera) PortPath() string { return c.sess.PortPath() }

// ListFolders 列出指定目录下的子目录。根目录为 "/"。
func (c *Camera) ListFolders(ctx context.Context, folder string) ([]Folder, error) {
	fs, err := c.app.ListFolders(ctx, folder)
	if err != nil {
		return nil, err
	}
	return toFolders(fs), nil
}

// ListFiles 列出指定目录下的文件（含文件信息）。
func (c *Camera) ListFiles(ctx context.Context, folder string) ([]File, error) {
	fs, err := c.app.ListFiles(ctx, folder)
	if err != nil {
		return nil, err
	}
	return toFiles(fs), nil
}

// ListTree 递归遍历相机全部目录与文件，用于预览所有相机内容。
func (c *Camera) ListTree(ctx context.Context) (*CameraContent, error) {
	folders, files, err := c.app.ListTree(ctx)
	if err != nil {
		return nil, err
	}
	return &CameraContent{
		Folders: toFolders(folders),
		Files:   toFiles(files),
	}, nil
}

// GetFileInfo 查询单个文件信息。
func (c *Camera) GetFileInfo(ctx context.Context, folder, name string) (File, error) {
	f, err := c.app.GetFileInfo(ctx, folder, name)
	if err != nil {
		return File{}, err
	}
	return toFile(f), nil
}

// DownloadTo 下载文件并写入 w（例如 os.File、bytes.Buffer）。
func (c *Camera) DownloadTo(ctx context.Context, folder, name string, w io.Writer) error {
	return c.app.DownloadTo(ctx, folder, name, w)
}

// GetInfo 聚合相机信息：摘要、驱动说明、存储、配置树。
func (c *Camera) GetInfo(ctx context.Context) (CameraInfo, error) {
	info, err := c.app.GetInfo(ctx)
	if err != nil {
		return CameraInfo{}, err
	}
	return toInfo(info), nil
}

// Close 断开当前相机会话。
func (c *Camera) Close(ctx context.Context) error { return c.app.Disconnect(ctx) }
