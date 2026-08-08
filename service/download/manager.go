package download

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"ci.zevix.uk/moment-go/pkg/moment"
	"ci.zevix.uk/moment-go/pkg/pubsub"
	"ci.zevix.uk/moment-go/pkg/task"
	"ci.zevix.uk/moment-go/pool/client"
	downloadv1 "ci.zevix.uk/moment-go/protogo/download/v1"
	"ci.zevix.uk/moment-go/storage"
)

// taskInstance 是 download 模块实例化后的泛型任务类型。
type taskInstance = task.Task[taskStatus, fileStatus, *downloadv1.MonitorTaskResponse]

// taskHandle 包装一个泛型任务并携带其任务 ID，向 download 服务暴露便捷 API。
type taskHandle struct {
	id    string
	inner *taskInstance
}

// taskID 返回任务唯一标识。
func (h *taskHandle) taskID() string { return h.id }

// inner 方法转发到泛型任务。

func (h *taskHandle) Pause()             { h.inner.Pause() }
func (h *taskHandle) Resume()            { h.inner.Resume() }
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
	mu    sync.Mutex
	tasks map[string]*taskHandle
}

var defaultManager = &manager{tasks: make(map[string]*taskHandle)}

func (m *manager) create(urls []string) (*taskHandle, error) {
	id, err := newTaskID()
	if err != nil {
		return nil, err
	}

	downloader := New(WithStorage(&storage.Disk{}))

	t := task.New(
		task.Config[taskStatus, fileStatus, *downloadv1.MonitorTaskResponse]{
			States:      taskState,
			Work:        makeWork(downloader),
			Build:       buildMonitorEvent(id),
			Jobs:        urls,
			Concurrency: 1,
		},
	)

	h := &taskHandle{id: id, inner: t}

	m.mu.Lock()
	m.tasks[id] = h
	m.mu.Unlock()

	// 任务结束后留一段宽限期供 MonitorTask 读取收尾事件，再回收内存。
	go func() {
		<-t.Done()
		time.Sleep(30 * time.Second)
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
