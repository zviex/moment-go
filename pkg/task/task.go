// Package task 提供一个与具体协议/实例解耦的、泛型的批量下载任务调度器。
//
// 它只依赖泛型类型参数（S/F 为整体/单文件的状态枚举，E 为事件类型），
// 状态枚举与事件类型均由具体使用方提供，因此本包不引入任何对
// protobuf、相机会话或具体事件消息的依赖：
//
//   - S (TaskStatus) / F (FileStatus)：状态枚举，通过 States 描述语义映射；
//   - E（事件，如 *downloadv1.MonitorTaskResponse）：任务经 pkg/pubsub 广播的
//     消息类型，由使用方通过 Config.Build 把内部状态变化转换/构造为具体事件
//     （即"具体实例自己实现的特殊逻辑"）。
package task

import (
	"container/list"
	"context"
	"errors"
	stdlibsync "sync"

	"ci.zevix.uk/moment-go/pkg/pubsub"
)

// EventKind 区分单文件事件与整体任务事件。
type EventKind int

const (
	// EventFile 表示单个文件的状态/进度变化。
	EventFile EventKind = iota
	// EventTask 表示整体任务的状态变化。
	EventTask
)

// Progress 描述单个文件的下载进度。
type Progress struct {
	// URL 该进度对应的文件引用。
	URL string
	// Downloaded 已下载字节数。
	Downloaded int64
	// Total 总字节数；-1 表示未知。
	Total int64
}

// Percent 返回 0-100 的百分比；总字节数未知时返回 -1。
func (p Progress) Percent() int32 {
	if p.Total <= 0 {
		return -1
	}
	if p.Downloaded <= 0 {
		return 0
	}
	if p.Downloaded >= p.Total {
		return 100
	}
	return int32(int64(100) * p.Downloaded / p.Total)
}

// Counts 是整体任务的计数快照。
type Counts struct {
	Completed int
	Failed    int
	Total     int
}

// Event 描述一次任务内部的状态变化：单文件事件或整体任务事件。
// 具体使用方的 BuildFunc 依据 Kind/各字段构造自己的事件消息。
type Event[S, F comparable] struct {
	Kind       EventKind
	URL        string
	FileStatus F
	TaskStatus S
	Progress   Progress
	Counts     Counts
	Error      string
}

// IsFile 是否为单文件事件。
func (e Event[S, F]) IsFile() bool { return e.Kind == EventFile }

// IsTask 是否为整体任务事件。
func (e Event[S, F]) IsTask() bool { return e.Kind == EventTask }

// States 把"语义状态"映射到使用方提供的具体枚举值，
// 使状态机不关心枚举的具体数值或命名。
type States[S, F comparable] struct {
	TaskRunning   S
	TaskPaused    S
	TaskCanceled  S
	TaskCompleted S
	TaskFailed    S

	FileDownloading F
	FileCompleted   F
	FileFailed      F
	FileCanceled    F
}

// WorkFunc 是单个文件的实际执行动作（例如经共享 Downloader 下载并落盘）。
type WorkFunc func(ctx context.Context, url string, onProgress func(Progress)) error

// BuildFunc 把内部状态变化（Event）转换为具体事件 E。
// 使用方在这里实现"自己的特殊逻辑"（例如构造 MonitorTaskResponse、
// 附加时间戳、区分事件类型等）。
type BuildFunc[S, F comparable, E any] func(ev Event[S, F]) E

// Config 配置一个任务。
type Config[S, F comparable, E any] struct {
	// States 提供状态枚举的语义映射。
	States States[S, F]
	// Work 执行单个文件。
	Work WorkFunc
	// Build 把内部事件构造为对外广播的具体事件。
	Build BuildFunc[S, F, E]
	// Jobs 初始待下载的 url 队列。
	Jobs []string
	// Concurrency 并发 worker 数；<=0 时缺省为 1。
	Concurrency int
	// RootCtx 是任务的父 ctx：取消时整个任务随之取消。为 nil 时退化为
	// context.Background()，任务只能通过 Cancel 终止。
	RootCtx context.Context
}

// Task 是一次批量下载任务：持有待下载队列、并发 worker 与暂停/恢复/取消/
// 移除队列等控制能力，并通过 pkg/pubsub 向订阅者广播 E 类型事件。
type Task[S, F comparable, E any] struct {
	cfg         Config[S, F, E]
	concurrency int
	bus         *pubsub.Bus[E]

	mu       stdlibsync.Mutex
	cond     *stdlibsync.Cond
	pending  *list.List
	inflight map[string]context.CancelFunc

	status    S
	paused    bool
	completed int
	failed    int
	total     int

	camCtx    context.Context
	cancelAll context.CancelFunc
	done      chan struct{}
	started   bool
}

// New 构造并启动一个任务。
func New[S, F comparable, E any](cfg Config[S, F, E]) *Task[S, F, E] {
	t := &Task[S, F, E]{
		cfg:         cfg,
		concurrency: cfg.Concurrency,
		bus:         pubsub.NewBus[E](),
		pending:     list.New(),
		inflight:    make(map[string]context.CancelFunc),
		status:      cfg.States.TaskRunning,
		total:       len(cfg.Jobs),
		done:        make(chan struct{}),
	}
	if t.concurrency <= 0 {
		t.concurrency = 1
	}
	t.cond = stdlibsync.NewCond(&t.mu)
	for _, j := range cfg.Jobs {
		t.pending.PushBack(j)
	}
	t.emitTask(t.status)
	t.Start()
	return t
}

// Bus 返回事件总线，供订阅（如 MonitorTask）订阅本任务事件。
func (t *Task[S, F, E]) Bus() *pubsub.Bus[E] { return t.bus }

// Status 返回当前整体任务状态。
func (t *Task[S, F, E]) Status() S {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

// Done 在任务完成/取消/失败时关闭，供外部等待收尾。
func (t *Task[S, F, E]) Done() <-chan struct{} { return t.done }

// Wait 阻塞直到任务完成，或 ctx 被取消。
func (t *Task[S, F, E]) Wait(ctx context.Context) error {
	select {
	case <-t.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start 启动后台 worker；重复调用无副作用。
func (t *Task[S, F, E]) Start() {
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return
	}
	t.started = true
	root := t.cfg.RootCtx
	if root == nil {
		root = context.Background()
	}
	t.camCtx, t.cancelAll = context.WithCancel(root)
	t.mu.Unlock()

	for i := 0; i < t.concurrency; i++ {
		go t.worker()
	}
	go t.watchRoot()
}

// watchRoot 在任务根 ctx 取消时取消整个任务，使阻塞在队列等待的 worker 及时退出。
func (t *Task[S, F, E]) watchRoot() {
	select {
	case <-t.camCtx.Done():
		t.Cancel()
	case <-t.done:
	}
}

// worker 是并发下载工作协程：不断从队列取下一个文件，直到任务结束。
func (t *Task[S, F, E]) worker() {
	for {
		url := t.next()
		if url == "" {
			return
		}
		t.runFile(url)
	}
}

// next 阻塞地从队列取出下一个待下载 url。返回 "" 表示 worker 应退出。
func (t *Task[S, F, E]) next() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	for {
		if t.status == t.cfg.States.TaskCanceled {
			return ""
		}
		for t.paused && t.status != t.cfg.States.TaskCanceled {
			t.cond.Wait()
		}
		if t.status == t.cfg.States.TaskCanceled {
			return ""
		}
		if front := t.pending.Front(); front != nil {
			url, _ := front.Value.(string)
			if url != "" {
				t.pending.Remove(front)
				return url
			}
		}
		if t.pending.Len() == 0 && len(t.inflight) == 0 &&
			t.status != t.cfg.States.TaskCanceled &&
			t.status != t.cfg.States.TaskPaused {
			return ""
		}
		t.cond.Wait()
	}
}

// runFile 顺序执行单个文件下载并广播对应事件。
func (t *Task[S, F, E]) runFile(url string) {
	fileCtx, cancel := context.WithCancel(t.camCtx)
	t.mu.Lock()
	t.inflight[url] = cancel
	t.mu.Unlock()

	t.emitFile(url, t.cfg.States.FileDownloading, Progress{URL: url, Downloaded: 0, Total: -1}, "")

	err := t.cfg.Work(fileCtx, url, func(p Progress) {
		t.emitProgress(url, p)
	})

	t.mu.Lock()
	delete(t.inflight, url)
	if err == nil {
		t.completed++
	} else {
		t.failed++
	}
	finished := t.pending.Len() == 0 && len(t.inflight) == 0
	status := t.status
	t.mu.Unlock()
	cancel()

	fileStatus := t.cfg.States.FileCompleted
	errMsg := ""
	switch {
	case isCancelled(err):
		fileStatus = t.cfg.States.FileCanceled
	case err != nil:
		fileStatus = t.cfg.States.FileFailed
		errMsg = err.Error()
	}
	t.emitFile(url, fileStatus, Progress{URL: url, Downloaded: 1, Total: 1}, errMsg)
	t.emitTask(status)

	if finished {
		t.finish()
	}
}

// emitProgress 广播单文件进行中的进度事件；从 Work 回调内安全调用。
func (t *Task[S, F, E]) emitProgress(url string, p Progress) {
	t.emitFile(url, t.cfg.States.FileDownloading, p, "")
}

// finish 在所有文件处理完毕后推进收尾状态并关闭 Done 通道。
func (t *Task[S, F, E]) finish() {
	t.mu.Lock()
	select {
	case <-t.done:
		t.mu.Unlock()
		return
	default:
	}
	status := t.cfg.States.TaskCompleted
	switch t.status {
	case t.cfg.States.TaskCanceled:
		status = t.cfg.States.TaskCanceled
	case t.cfg.States.TaskPaused:
		status = t.cfg.States.TaskPaused
	default:
		if t.failed > 0 {
			status = t.cfg.States.TaskFailed
		}
	}
	t.status = status
	close(t.done)
	t.mu.Unlock()
	if t.cancelAll != nil {
		t.cancelAll()
	}
	t.cond.Broadcast()
	t.emitTask(status)
}

func (t *Task[S, F, E]) counts() Counts {
	return Counts{Completed: t.completed, Failed: t.failed, Total: t.total}
}

func (t *Task[S, F, E]) emit(ev Event[S, F]) {
	t.bus.Publish(t.cfg.Build(ev))
}

// emitFile 广播单个文件事件；内部加锁读取当前状态快照，避免与并发控制操作竞争。
func (t *Task[S, F, E]) emitFile(url string, fs F, p Progress, errMsg string) {
	t.mu.Lock()
	status := t.status
	counts := t.counts()
	t.mu.Unlock()
	t.emit(Event[S, F]{
		Kind:       EventFile,
		URL:        url,
		FileStatus: fs,
		TaskStatus: status,
		Progress:   p,
		Counts:     counts,
		Error:      errMsg,
	})
}

// emitTask 广播整体任务事件；内部加锁读取当前状态快照。
func (t *Task[S, F, E]) emitTask(status S) {
	t.mu.Lock()
	counts := t.counts()
	t.mu.Unlock()
	t.emit(Event[S, F]{
		Kind:       EventTask,
		TaskStatus: status,
		Counts:     counts,
	})
}

// Snapshot 发布一次当前整体状态的快照事件（供订阅者在任意时刻获得最新状态）。
func (t *Task[S, F, E]) Snapshot() {
	t.emitTask(t.Status())
}

// ---- 控制接口 ----

// Pause 暂停整体任务：正在进行的文件允许收尾，之后 worker 阻塞等待恢复。
func (t *Task[S, F, E]) Pause() {
	t.mu.Lock()
	if t.paused || t.status == t.cfg.States.TaskCanceled || t.status == t.cfg.States.TaskCompleted {
		t.mu.Unlock()
		return
	}
	t.paused = true
	t.status = t.cfg.States.TaskPaused
	t.mu.Unlock()
	t.emitTask(t.status)
}

// Resume 恢复被暂停的整体任务。
func (t *Task[S, F, E]) Resume() {
	t.mu.Lock()
	if !t.paused {
		t.mu.Unlock()
		return
	}
	t.paused = false
	t.status = t.cfg.States.TaskRunning
	t.cond.Broadcast()
	t.mu.Unlock()
	t.emitTask(t.status)
}

// Cancel 取消整个任务：清空队列并中断所有正在下载的文件。
func (t *Task[S, F, E]) Cancel() {
	t.mu.Lock()
	if t.status == t.cfg.States.TaskCanceled || t.status == t.cfg.States.TaskCompleted {
		t.mu.Unlock()
		return
	}
	t.status = t.cfg.States.TaskCanceled
	t.pending.Init()
	cancels := make([]context.CancelFunc, 0, len(t.inflight))
	for _, c := range t.inflight {
		cancels = append(cancels, c)
	}
	t.cond.Broadcast()
	t.mu.Unlock()

	for _, c := range cancels {
		c()
	}
	if t.cancelAll != nil {
		t.cancelAll()
	}
	t.emitTask(t.status)

	t.mu.Lock()
	select {
	case <-t.done:
	default:
		close(t.done)
	}
	t.mu.Unlock()
}

// RemoveFromQueue 把还在队列中等待下载的 url 移除（标记为已取消），
// 对正在下载的文件则直接取消其传输。返回是否找到并处理。
func (t *Task[S, F, E]) RemoveFromQueue(url string) bool {
	t.mu.Lock()
	for e := t.pending.Front(); e != nil; e = e.Next() {
		if s, ok := e.Value.(string); ok && s == url {
			t.pending.Remove(e)
			t.cond.Broadcast()
			t.mu.Unlock()
			t.emitFile(url, t.cfg.States.FileCanceled, Progress{URL: url}, "removed from queue")
			return true
		}
	}
	if cancel, ok := t.inflight[url]; ok {
		t.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		t.emitFile(url, t.cfg.States.FileCanceled, Progress{URL: url}, "canceled")
		return true
	}
	t.mu.Unlock()
	return false
}

// isCancelled 判断错误是否为取消类错误。
func isCancelled(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
