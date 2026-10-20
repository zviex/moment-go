package moment

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Client struct {
	connector  Connector
	discoverer Discoverer

	// ops 串行化 Connect / Disconnect，避免生命周期竞争。
	ops sync.Mutex

	mu      sync.RWMutex
	state   State
	lastErr error
	session Session
}

func NewClient(connector Connector, discoverer Discoverer) *Client {
	return &Client{
		connector:  connector,
		discoverer: discoverer,
		state:      StateDisconnected,
	}
}

// State 返回当前生命周期状态。
func (c *Client) State() State {
	if c == nil {
		return StateDisconnected
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// IsConnected 是否已连接。
func (c *Client) IsConnected() bool { return c.State() == StateConnected }

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
func (c *Client) Detect(ctx context.Context) ([]DetectedCamera, error) {
	if c == nil || c.discoverer == nil {
		return nil, ErrBackendUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.discoverer.Detect(ctx)
}

// Connect 建立相机连接，成功时返回当前 *Camera。
// 若已连接则直接复用当前会话，不重复建立新连接。
func (c *Client) Connect(ctx context.Context, opts ConnectOptions) (*Camera, error) {
	if c == nil || c.connector == nil {
		return nil, ErrBackendUnavailable
	}

	c.ops.Lock()
	defer c.ops.Unlock()

	switch c.State() {
	case StateConnected:
		sess, err := c.sessionOrErr()
		if err != nil {
			return nil, err
		}
		return &Camera{client: c, session: sess}, nil
	case StateConnecting, StateDisconnecting:
		return nil, ErrInvalidState
	}

	c.setState(StateConnecting)

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	slog.Info("connecting camera",
		"type", opts.Type, "model", opts.Model, "host", opts.Host, "port", opts.Port)

	sess, err := c.connector.Connect(ctx, opts)
	if err != nil {
		c.fail(err)
		slog.Error("connect failed", "err", err)
		return nil, err
	}

	c.mu.Lock()
	c.session = sess
	c.state = StateConnected
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
	case StateDisconnected:
		return nil
	case StateConnecting, StateDisconnecting:
		return ErrInvalidState
	}

	c.mu.Lock()
	sess := c.session
	c.session = nil
	c.state = StateDisconnecting
	c.mu.Unlock()

	var closeErr error
	if sess != nil {
		closeErr = sess.Close(ctx)
	}

	c.mu.Lock()
	c.state = StateDisconnected
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

	if closer, ok := c.connector.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func (c *Client) sessionOrErr() (Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.state != StateConnected || c.session == nil {
		return nil, ErrNotConnected
	}
	return c.session, nil
}

func (c *Client) setState(s State) {
	c.mu.Lock()
	c.state = s
	c.mu.Unlock()
}

func (c *Client) fail(err error) {
	c.mu.Lock()
	c.state = StateDisconnected
	c.lastErr = err
	c.mu.Unlock()
}
