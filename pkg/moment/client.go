package moment

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"ci.zevix.uk/moment-go/adapter"
	"ci.zevix.uk/moment-go/adapter/ports"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
)

var _ ports.ConnectionManager = (*Client)(nil)

// Client 是相机会话的唯一所有者：负责连接、断开与在途操作的租约管理。
// 数据面能力由 capability.go 中的代理通过 withSession 借用当前会话。
type Client struct {
	connect    adapter.ConnectFunc
	discoverer adapter.Discoverer
	closeFn    func() error

	// ops 串行化 Connect / Disconnect，避免生命周期竞争。
	ops sync.Mutex

	mu            sync.Mutex
	state         camerav1.State
	lastErr       error
	session       adapter.Session
	sessionCtx    context.Context
	sessionCancel context.CancelFunc
	refs          int
	idle          *sync.Cond
}

func NewClient(connect adapter.ConnectFunc, discoverer adapter.Discoverer, closeFn ...func() error) *Client {
	c := &Client{
		connect:    connect,
		discoverer: discoverer,
		state:      camerav1.State_STATE_DISCONNECTED,
	}
	c.idle = sync.NewCond(&c.mu)
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// IsConnected 是否已连接。
func (c *Client) IsConnected() bool { return c.State() == camerav1.State_STATE_CONNECTED }

// LastError 返回最近一次失败原因。
func (c *Client) LastError() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
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
// 返回连接元信息而非会话句柄，避免调用方持有可逃逸的生命周期对象。
func (c *Client) Connect(ctx context.Context, target *camerav1.Tartget) (ports.ConnectionInfo, error) {
	var zero ports.ConnectionInfo
	if c == nil || c.connect == nil {
		return zero, ErrBackendUnavailable
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	c.ops.Lock()
	defer c.ops.Unlock()

	switch c.State() {
	case camerav1.State_STATE_CONNECTED:
		sess, err := c.sessionOrErr()
		if err != nil {
			return zero, err
		}
		return connectionInfo(sess), nil
	case camerav1.State_STATE_CONNECTING, camerav1.State_STATE_DISCONNECTING:
		return zero, ErrInvalidState
	}

	c.setState(camerav1.State_STATE_CONNECTING)

	slog.Info("connecting camera", "port_path", target.PortPath, "model", target.Model)

	sess, err := c.connect(ctx, target)
	if err != nil {
		c.fail(err)
		slog.Error("connect failed", "err", err)
		return zero, err
	}

	sessionCtx, sessionCancel := context.WithCancel(context.Background())

	c.mu.Lock()
	c.session = sess
	c.sessionCtx = sessionCtx
	c.sessionCancel = sessionCancel
	c.state = camerav1.State_STATE_CONNECTED
	c.lastErr = nil
	c.mu.Unlock()

	slog.Info("connected", "model", sess.Model(), "driver", sess.Driver(), "port", sess.PortPath())
	return connectionInfo(sess), nil
}

// Disconnect 取消在途操作、等待租约释放并关闭底层会话。
func (c *Client) Disconnect(ctx context.Context) error {
	if c == nil {
		return nil
	}

	c.ops.Lock()
	defer c.ops.Unlock()

	c.mu.Lock()
	switch c.state {
	case camerav1.State_STATE_CONNECTED:
	case camerav1.State_STATE_CONNECTING, camerav1.State_STATE_DISCONNECTING:
		c.mu.Unlock()
		return ErrInvalidState
	default:
		c.mu.Unlock()
		return nil
	}

	c.state = camerav1.State_STATE_DISCONNECTING
	sess := c.session
	cancel := c.sessionCancel
	c.sessionCancel = nil
	c.mu.Unlock()

	// 立即取消在途操作与拍摄事件流，使它们释放租约。
	if cancel != nil {
		cancel()
	}

	c.mu.Lock()
	if c.refs > 0 {
		slog.Debug("disconnect: waiting for in-flight operations", "refs", c.refs)
	}
	for c.refs > 0 {
		c.idle.Wait()
	}
	c.session = nil
	c.sessionCtx = nil
	c.mu.Unlock()

	if sess != nil {
		slog.Debug("disconnecting camera", "port_path", sess.PortPath(), "driver", sess.Driver())
	}

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
//
// Disconnect 内部获取 c.ops、等待在途租约时都无法被 ctx 中断，因此这里
// 额外加一个硬超时：进程退出阶段不再无限等待卡在 cgo 里的相机操作。
// 超时后刻意不调用 closeFn，避免在 cgo 仍使用 GPContext 时释放它。
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	slog.Debug("client close: disconnecting")
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Disconnect(ctx)
		close(done)
	}()

	select {
	case <-done:
		slog.Debug("client close: disconnect completed")
	case <-time.After(5 * time.Second):
		slog.Warn("client close: timed out waiting for in-flight camera operations; " +
			"skip backend close to avoid use-after-free")
		return nil
	}

	if c.closeFn != nil {
		return c.closeFn()
	}
	return nil
}

// withSession 在租约内执行一次短操作。fn 收到的 ctx 会在断开时被取消。
func (c *Client) withSession(ctx context.Context, fn func(context.Context, adapter.Session) error) error {
	sess, sessionCtx, release, err := c.borrowSession(ctx)
	if err != nil {
		return err
	}
	defer release()

	opCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(sessionCtx, cancel)
	defer func() {
		stop()
		cancel()
	}()

	if err := opCtx.Err(); err != nil {
		return err
	}
	return fn(opCtx, sess)
}

// borrowSession 增加一个租约并返回当前会话。调用方必须在用完后调用 release。
// 拍摄事件这类长生命周期能力会持有租约直到事件流结束。
func (c *Client) borrowSession(ctx context.Context) (adapter.Session, context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	if c == nil {
		return nil, nil, nil, ErrNotConnected
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != camerav1.State_STATE_CONNECTED || c.session == nil || c.sessionCtx == nil {
		return nil, nil, nil, ErrNotConnected
	}

	c.refs++
	sess, sessionCtx := c.session, c.sessionCtx
	release := sync.OnceFunc(func() {
		c.mu.Lock()
		c.refs--
		if c.refs == 0 {
			c.idle.Broadcast()
		}
		c.mu.Unlock()
	})
	return sess, sessionCtx, release, nil
}

func (c *Client) sessionOrErr() (adapter.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != camerav1.State_STATE_CONNECTED || c.session == nil {
		return nil, ErrNotConnected
	}
	return c.session, nil
}

func connectionInfo(sess adapter.Session) ports.ConnectionInfo {
	return ports.ConnectionInfo{
		Model:    sess.Model(),
		Driver:   sess.Driver(),
		PortPath: sess.PortPath(),
	}
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
