package download

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"ci.zevix.uk/moment-go/adapter/ports"
	"ci.zevix.uk/moment-go/pkg/pubsub"
	"ci.zevix.uk/moment-go/pkg/task"
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

// Manager 是任务注册表：CreateTask 创建并登记任务，MonitorTask/ControlTask
// 通过 taskID 定位任务。
type Manager struct {
	mu         sync.Mutex
	tasks      map[string]*taskHandle
	rootCtx    context.Context
	store      *Store
	downloader ports.Downloader
}

func NewManager(rootCtx context.Context, store *Store, downloader ports.Downloader) *Manager {
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	return &Manager{
		tasks:      make(map[string]*taskHandle),
		rootCtx:    rootCtx,
		store:      store,
		downloader: downloader,
	}
}

func (m *Manager) getStore() *Store {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store
}

// persistTimeout 限制单次落库耗时，避免数据库异常拖慢下载事件广播。
const persistTimeout = 5 * time.Second

// persist 把任务内部事件写入历史库。失败只记日志，不影响下载本身。
func (m *Manager) persist(taskID string, ev task.Event[taskStatus, fileStatus]) {
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
func (m *Manager) history(ctx context.Context, id string) (*TaskRecord, error) {
	s := m.getStore()
	if s == nil {
		return nil, ErrTaskNotFound
	}
	return s.GetTask(ctx, id)
}

func (m *Manager) root(fallback context.Context) context.Context {
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

func (m *Manager) create(ctx context.Context, urls []string) (*taskHandle, error) {
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

	toEvent := buildMonitorEvent(id)

	t := task.New(
		task.Config[taskStatus, fileStatus, *downloadv1.MonitorTaskResponse]{
			States: taskState,
			Work:   makeWork(m.downloader),
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

func (m *Manager) get(id string) (*taskHandle, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	return t, ok
}

func (m *Manager) remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, id)
}

// taskID 返回任务唯一标识。
func (h *taskHandle) taskID() string { return h.id }

// makeWork 把共享的 Downloader 包装成泛型任务要求的 WorkFunc。
func makeWork(d ports.Downloader) task.WorkFunc {
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
