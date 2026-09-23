package sync

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"ci.zevix.uk/moment-go/service/download"
)

var errNoSource = errors.New("autosync: capture source not set")

// CaptureEvent 表示相机一次拍摄产生的一个新文件。
type CaptureEvent struct {
	URL string
	At  time.Time
}

type CaptureSource interface {
	// Events 返回拍摄事件通道；ctx 取消时该通道关闭。
	Events(ctx context.Context) (<-chan CaptureEvent, error)
}

// Recorder 并发安全由 run 的 atomic.Pointer CAS/Swap 保证，内部无锁。
type Recorder struct {
	// 构造期注入，之后只读。rootCtx 为应用级根 ctx：其取消会终止自动同步。
	rootCtx    context.Context
	downloader download.Downloader
	source     CaptureSource

	// 运行态：nil = disabled；非 nil = 正在运行。
	run atomic.Pointer[recorderRun]
}

type recorderRun struct {
	cancel context.CancelFunc
}

// NewRecorder 构造 Recorder：rootCtx 为应用级根 ctx，source/downloader 构造期注入。
func NewRecorder(rootCtx context.Context, source CaptureSource, downloader download.Downloader) *Recorder {
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	return &Recorder{rootCtx: rootCtx, source: source, downloader: downloader}
}

// NewDefaultRecorder 构造使用共享 Downloader 的 Recorder。
func NewDefaultRecorder(rootCtx context.Context, source CaptureSource) *Recorder {
	return NewRecorder(rootCtx, source, download.New())
}

// defaultRecorder 为全局共享实例；rootCtx/source 由 Init 阶段注入。
var defaultRecorder = NewDefaultRecorder(nil, nil)

// Enable 开启自动同步：开始监听拍摄事件并下载每个新拍文件。
// 已在运行时重复调用无副作用。未注入 source 时返回错误。
func (r *Recorder) Enable() error {
	if r.source == nil {
		return errNoSource
	}

	ctx, cancel := context.WithCancel(r.rootCtx)

	newRun := &recorderRun{cancel: cancel}
	if !r.run.CompareAndSwap(nil, newRun) {
		// 已在运行：释放刚创建的 ctx 未用资源，无副作用。
		cancel()
		return nil
	}

	go r.runLoop(ctx)

	return nil
}

// Disable 关闭自动同步，中断正在监听的事件流。幂等。
func (r *Recorder) Disable() {
	if run := r.run.Swap(nil); run != nil {
		run.cancel()
	}
}

// Enabled 返回自动同步是否开启。
func (r *Recorder) Enabled() bool {
	return r.run.Load() != nil
}

// runLoop 消费拍摄事件并逐个下载，直到 ctx 取消。
// 仅在 CAS 抢占成功后启动，ctx 由 Enable 创建、由 Disable 经 Swap-cancel 关闭。
func (r *Recorder) runLoop(ctx context.Context) {
	ch, err := r.source.Events(ctx)
	if err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}

			r.syncFile(ctx, ev)
		}
	}
}

// syncFile 把单个拍摄事件对应的文件同步到 Disk。
func (r *Recorder) syncFile(parent context.Context, ev CaptureEvent) {
	if ev.URL == "" {
		return
	}

	d := r.downloader
	if d == nil {
		return
	}

	_ = d.Download(parent, ev.URL, nil)
}
