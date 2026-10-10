package download

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"ci.zevix.uk/moment-go/pkg/moment"
	"ci.zevix.uk/moment-go/pkg/pubsub"
	"ci.zevix.uk/moment-go/pkg/task"
	"ci.zevix.uk/moment-go/pool/client"
	"ci.zevix.uk/moment-go/storage"
	downloadv1 "ci.zevix.uk/moment/moment-protogo/download/v1"
)

// taskInstance 是 download 模块实例化后的泛型任务类型。
type taskInstance = task.Task[taskStatus, fileStatus, *downloadv1.MonitorTaskResponse]

// taskHandle 包装一个泛型任务并携带其任务 ID，向 download 服务暴露便捷 API。
type taskHandle struct {
	id    string
	inner *taskInstance
}

// inner 方法转发到泛型任务。

func (h *taskHandle) Pause()             { h.inner.Pause() }
func (h *taskHandle) Cancel()            { h.inner.Cancel() }
func (h *taskHandle) Status() taskStatus { return h.inner.Status() }
func (h *taskHandle) Snapshot()          { h.inner.Snapshot() }
func (h *taskHandle) Done() <-chan struct{} {
	return h.inner.Done()
}
func (h *taskHandle) RemoveFromQueue(url string) bool { return h.inner.RemoveFromQueue(url) }
func (h *taskHandle) SubscribeEvents() *pubsub.Subscriber[*downloadv1.MonitorTaskResponse] {
	return h.inner.Bus().Subscribe()
}

// manager 是任务注册表：CreateTask 创建并登记任务，MonitorTask/ControlTask
// 通过 taskID 定位任务。
type manager struct {
	mu      sync.Mutex
	tasks   map[string]*taskHandle
	rootCtx context.Context
	// store 为任务历史的持久化后端；为 nil 时不做持久化。
	store *Store
}

var defaultManager = &manager{tasks: make(map[string]*taskHandle)}

// SetStore 绑定历史持久化存储。
func (m *manager) SetStore(s *Store) {
	m.mu.Lock()
	m.store = s
	m.mu.Unlock()
}

func (m *manager) getStore() *Store {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store
}

// persistTimeout 限制单次落库耗时，避免数据库异常拖慢下载事件广播。
const persistTimeout = 5 * time.Second

// persist 把任务内部事件写入历史库。失败只记日志，不影响下载本身。
func (m *manager) persist(taskID string, ev task.Event[taskStatus, fileStatus]) {
	s := m.getStore()
	if s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
	defer cancel()
	var err error
	switch {
	case ev.IsFile():
		pct := ev.Progress.Percent()
		if pct < 0 {
			pct = 0
		}
		err = s.RecordFile(ctx, taskID, ev.URL, ev.FileStatus, pct,
			ev.Progress.Downloaded, ev.Progress.Total, ev.Error)
	case ev.IsTask():
		err = s.RecordTask(ctx, taskID, ev.TaskStatus, ev.Counts.Completed, ev.Counts.Failed)
	}
	if err != nil {
		slog.Warn("download history: persist event failed", "task", taskID, "err", err)
	}
}

// history 从历史库读取一个已结束（或已中断）的任务。
func (m *manager) history(ctx context.Context, id string) (*TaskRecord, error) {
	return m.getStore().GetTask(ctx, id)
}

// SetRootContext 绑定 manager 的应用级根 ctx：新建任务挂到该 ctx 下，
// 应用退出时取消根 ctx 即可终止所有任务与回收协程。
func (m *manager) SetRootContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	m.rootCtx = ctx
	m.mu.Unlock()
}

// root 返回 manager 的根 ctx；未设置时退化为 ctx（通常是调用方传入的请求 ctx），
// 最终兜底为 Background。
func (m *manager) root(fallback context.Context) context.Context {
	m.mu.Lock()
	root := m.rootCtx
	m.mu.Unlock()
	if root != nil {
		return root
	}
	if fallback != nil {
		return fallback
	}
	return context.Background()
}

func (m *manager) create(ctx context.Context, urls []string) (*taskHandle, error) {
	id, err := newTaskID()
	if err != nil {
		return nil, err
	}

	root := m.root(ctx)

	// 先登记历史记录，保证之后的事件都能找到对应行。登记失败不阻断下载。
	if s := m.getStore(); s != nil {
		cctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
		if err := s.CreateTask(cctx, id, urls); err != nil {
			slog.Warn("download history: create task failed", "task", id, "err", err)
		}
		cancel()
	}

	downloader := New(WithStorage(storage.Default()))
	toEvent := buildMonitorEvent(id)

	t := task.New(
		task.Config[taskStatus, fileStatus, *downloadv1.MonitorTaskResponse]{
			States: taskState,
			Work:   makeWork(downloader),
			Build: func(ev task.Event[taskStatus, fileStatus]) *downloadv1.MonitorTaskResponse {
				m.persist(id, ev)
				return toEvent(ev)
			},
			Jobs:        urls,
			Concurrency: 1,
			RootCtx:     root,
		},
	)

	h := &taskHandle{id: id, inner: t}

	m.mu.Lock()
	m.tasks[id] = h
	m.mu.Unlock()

	// 任务结束后留一段宽限期供 MonitorTask 读取收尾事件，再回收内存；
	// 宽限期内根 ctx 取消则立即回收。
	go func() {
		select {
		case <-t.Done():
		case <-root.Done():
			m.remove(id)
			return
		}
		select {
		case <-time.After(30 * time.Second):
		case <-root.Done():
		}
		m.remove(id)
	}()

	return h, nil
}

func (m *manager) get(id string) (*taskHandle, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	return t, ok
}

func (m *manager) remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, id)
}

// taskID 返回任务唯一标识。
func (h *taskHandle) taskID() string { return h.id }

// makeWork 把共享的 Downloader 包装成泛型任务要求的 WorkFunc。
func makeWork(d Downloader) task.WorkFunc {
	return func(ctx context.Context, url string, onProgress func(task.Progress)) error {
		if d == nil {
			return ErrNoCamera
		}
		return d.Download(ctx, url, func(p task.Progress) {
			onProgress(p)
		})
	}
}

func newTaskID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// momentClientCamera 从客户端单例拿到当前相机会话，供默认下载器使用。
func momentClientCamera() (*moment.Camera, error) {
	return client.Default().Camera()
}
