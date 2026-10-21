package moment

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"ci.zevix.uk/moment-go/adapter"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

type Client struct {
	connect    adapter.ConnectFunc
	discoverer adapter.Discoverer
	closeFn    func() error

	// ops 串行化 Connect / Disconnect，避免生命周期竞争。
	ops sync.Mutex

	mu      sync.RWMutex
	state   camerav1.State
	lastErr error
	session adapter.Session
}

func NewClient(connect adapter.ConnectFunc, discoverer adapter.Discoverer, closeFn ...func() error) *Client {
	c := &Client{
		connect:    connect,
		discoverer: discoverer,
		state:      camerav1.State_STATE_DISCONNECTED,
	}
	if len(closeFn) > 0 {
		c.closeFn = closeFn[0]
	}
	return c
}

// State 返回当前生命周期状态。
func (c *Client) State() camerav1.State {
	if c == nil {
		return camerav1.State_STATE_DISCONNECTED
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// IsConnected 是否已连接。
func (c *Client) IsConnected() bool { return c.State() == camerav1.State_STATE_CONNECTED }

// LastError 返回最近一次失败原因。
func (c *Client) LastError() error {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastErr
}

// Detect 探测可用相机（USB 自动检测）。
func (c *Client) Detect(ctx context.Context) ([]adapter.DetectedCamera, error) {
	if c == nil || c.discoverer == nil {
		return nil, ErrBackendUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.discoverer.Detect(ctx)
}

// Connect 若已连接则直接复用当前会话，不重复建立新连接。
// 连接超时由调用方通过 context 控制。
func (c *Client) Connect(ctx context.Context, target *camerav1.Tartget) (*Camera, error) {
	if c == nil || c.connect == nil {
		return nil, ErrBackendUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.ops.Lock()
	defer c.ops.Unlock()

	switch c.State() {
	case camerav1.State_STATE_CONNECTED:
		sess, err := c.sessionOrErr()
		if err != nil {
			return nil, err
		}
		return &Camera{client: c, session: sess}, nil
	case camerav1.State_STATE_CONNECTING, camerav1.State_STATE_DISCONNECTING:
		return nil, ErrInvalidState
	}

	c.setState(camerav1.State_STATE_CONNECTING)

	slog.Info("connecting camera", "port_path", target.PortPath, "model", target.Model)

	sess, err := c.connect(ctx, target)
	if err != nil {
		c.fail(err)
		slog.Error("connect failed", "err", err)
		return nil, err
	}

	c.mu.Lock()
	c.session = sess
	c.state = camerav1.State_STATE_CONNECTED
	c.lastErr = nil
	c.mu.Unlock()

	slog.Info("connected", "model", sess.Model(), "driver", sess.Driver(), "port", sess.PortPath())
	return &Camera{client: c, session: sess}, nil
}

// Camera 返回当前已连接会话对应的 Camera；未连接时返回 ErrNotConnected。
func (c *Client) Camera() (*Camera, error) {
	sess, err := c.sessionOrErr()
	if err != nil {
		return nil, err
	}
	return &Camera{client: c, session: sess}, nil
}

// Disconnect 断开连接并释放会话资源。
func (c *Client) Disconnect(ctx context.Context) error {
	if c == nil {
		return nil
	}

	c.ops.Lock()
	defer c.ops.Unlock()

	switch c.State() {
	case camerav1.State_STATE_CONNECTED:
		return nil
	case camerav1.State_STATE_CONNECTING, camerav1.State_STATE_DISCONNECTING:
		return ErrInvalidState
	}

	c.mu.Lock()
	sess := c.session
	c.session = nil
	c.state = camerav1.State_STATE_DISCONNECTING
	c.mu.Unlock()

	var closeErr error
	if sess != nil {
		closeErr = sess.Close(ctx)
	}

	c.mu.Lock()
	c.state = camerav1.State_STATE_DISCONNECTED
	c.lastErr = closeErr
	c.mu.Unlock()

	if closeErr != nil {
		slog.Warn("disconnect", "err", closeErr)
	} else {
		slog.Info("disconnected")
	}
	return closeErr
}

// Close 断开连接并释放后端资源（若后端实现了 io.Closer）。
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = c.Disconnect(ctx)

	if c.closeFn != nil {
		return c.closeFn()
	}
	return nil
}

func (c *Client) sessionOrErr() (adapter.Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state != camerav1.State_STATE_CONNECTED || c.session == nil {
		return nil, ErrNotConnected
	}
	return c.session, nil
}

func (c *Client) setState(s camerav1.State) {
	c.mu.Lock()
	c.state = s
	c.mu.Unlock()
}

func (c *Client) fail(err error) {
	c.mu.Lock()
	c.state = camerav1.State_STATE_DISCONNECTED
	c.lastErr = err
	c.mu.Unlock()
}
