package sync

import (
	"context"
	"errors"
	"sync/atomic"

	"ci.zevix.uk/moment-go/adapter/ports"
)

var errNoSource = errors.New("autosync: capture source not set")

// Recorder 并发安全由 run 的 atomic.Pointer CAS/Swap 保证，内部无锁。
type Recorder struct {
	rootCtx context.Context

	downloader ports.Downloader
	source     ports.CaptureSource

	// 运行态：nil = disabled；非 nil = 正在运行。
	run atomic.Pointer[recorderRun]
}

type recorderRun struct {
	cancel context.CancelFunc
}

func NewRecorder(rootCtx context.Context, source ports.CaptureSource, downloader ports.Downloader) *Recorder {
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	return &Recorder{rootCtx: rootCtx, source: source, downloader: downloader}
}

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

	go func() {
		r.runLoop(ctx)
		// 事件流结束（连接断开或源关闭）后回到 disabled，允许重新 Enable。
		r.run.CompareAndSwap(newRun, nil)
		cancel()
	}()

	return nil
}

// Disable 关闭自动同步，中断正在监听的事件流。幂等。
func (r *Recorder) Disable() {
	if run := r.run.Swap(nil); run != nil {
		run.cancel()
	}
}

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
func (r *Recorder) syncFile(parent context.Context, ev ports.CaptureEvent) {
	if ev.URL == "" {
		return
	}

	d := r.downloader
	if d == nil {
		return
	}

	_ = d.Download(parent, ev.URL, nil)
}
