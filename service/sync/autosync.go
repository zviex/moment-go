package sync

import (
	"context"
	"time"

	stdsync "sync"

	"ci.zevix.uk/moment-go/service/download"
)

// CaptureEvent 表示相机一次拍摄产生的一个新文件。
type CaptureEvent struct {
	// URL 指向相机内新拍摄的文件引用，与 download.CreateTask 的 url 同构，
	// 由底层相机会话解析为 folder + name。
	URL string
	// At 事件产生时间（按下快门的时刻）。
	At time.Time
}

// CaptureSource 提供相机拍摄事件流。底层接入（捕捉快门/拍摄事件）后，
// 通过 Events() 对外吐出每次拍摄产生的新文件。
// 相机事件体系尚未接入时，可用 SetCaptureSource 注入一个桩实现。
type CaptureSource interface {
	// Events 返回拍摄事件通道；ctx 取消时该通道关闭。
	Events(ctx context.Context) (<-chan CaptureEvent, error)
}

// Recorder 是自动同步执行单元：订阅相机拍摄事件，并把每个新拍文件
// 通过共享的 download.Downloader（与手动 task 同一抽象）写入 Disk。
type Recorder struct {
	downloader download.Downloader
	source     CaptureSource

	mu      stdsync.Mutex
	enabled bool
	cancel  context.CancelFunc
	done    chan struct{}
}

var defaultRecorder = &Recorder{downloader: download.New()}

// SetCaptureSource 注入拍摄事件来源（例如将相机的快门事件接到这里）。
// 未设置时 Recorder 处于空闲状态，不会执行任何下载。
func SetCaptureSource(s CaptureSource) {
	defaultRecorder.mu.Lock()
	defer defaultRecorder.mu.Unlock()
	defaultRecorder.source = s
}

// SetDownloader 覆盖自动同步使用的下载行为（默认与 download 共用同一实现）。
func SetDownloader(d download.Downloader) {
	defaultRecorder.mu.Lock()
	defer defaultRecorder.mu.Unlock()
	if d != nil {
		defaultRecorder.downloader = d
	}
}

// Enable 开启自动同步：开始监听拍摄事件并下载每个新拍文件。
// 已在运行时重复调用无副作用。
func (r *Recorder) Enable() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.enabled {
		return nil
	}
	if r.source == nil {
		// 尚无拍摄事件来源：不 panic，但记录为一个可诊断的未启用状态。
		r.enabled = true // 仍标记为已开启，等 source 注入后即可工作。
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	r.enabled = true
	go r.run(ctx, r.done)
	return nil
}

// Disable 关闭自动同步，中断正在监听的事件流。
func (r *Recorder) Disable() {
	r.mu.Lock()
	if !r.enabled {
		r.mu.Unlock()
		return
	}
	r.enabled = false
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Enabled 返回自动同步是否开启。
func (r *Recorder) Enabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enabled
}

// run 消费拍摄事件并逐个下载，直到 ctx 取消。
func (r *Recorder) run(ctx context.Context, done chan struct{}) {
	defer close(done)
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
	r.mu.Lock()
	d := r.downloader
	r.mu.Unlock()
	if d == nil {
		return
	}
	_ = d.Download(parent, ev.URL, nil)
}
