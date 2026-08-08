package task_test

import (
	"context"
	"errors"
	stdstd "sync"
	"testing"
	"time"

	"ci.zevix.uk/moment-go/pkg/task"
)

// -- 具体的状态枚举（模拟业务侧提供的枚举，验证泛型解耦）--

type ts int

const (
	tsRunning ts = iota
	tsPaused
	tsCanceled
	tsCompleted
	tsFailed
)

type fs int

const (
	fsDownloading fs = iota
	fsCompleted
	fsFailed
	fsCanceled
)

type testEvent struct {
	url      string
	file     fs
	task     ts
	progress task.Progress
	err      string
	isTask   bool
}

var testStates = task.States[ts, fs]{
	TaskRunning:   tsRunning,
	TaskPaused:    tsPaused,
	TaskCanceled:  tsCanceled,
	TaskCompleted: tsCompleted,
	TaskFailed:    tsFailed,

	FileDownloading: fsDownloading,
	FileCompleted:   fsCompleted,
	FileFailed:      fsFailed,
	FileCanceled:    fsCanceled,
}

func buildTestEvent(e task.Event[ts, fs]) testEvent {
	return testEvent{
		url:      e.URL,
		file:     e.FileStatus,
		task:     e.TaskStatus,
		progress: e.Progress,
		err:      e.Error,
		isTask:   e.IsTask(),
	}
}

func newTestTask(st task.States[ts, fs], urls []string, work task.WorkFunc, n int) *task.Task[ts, fs, testEvent] {
	return task.New(
		task.Config[ts, fs, testEvent]{
			States:      st,
			Work:        work,
			Build:       buildTestEvent,
			Jobs:        urls,
			Concurrency: n,
		},
	)
}

// fakeWork 是 WorkFunc 测试替身：可配置延迟与失败 url。
type fakeWork struct {
	mu      stdstd.Mutex
	delay   time.Duration
	done    map[string]int
	started map[string]int
	failURL map[string]error
}

func newFakeWork() *fakeWork {
	return &fakeWork{
		done:    make(map[string]int),
		started: make(map[string]int),
		failURL: make(map[string]error),
	}
}

func (f *fakeWork) run() task.WorkFunc {
	return func(ctx context.Context, url string, onProgress func(task.Progress)) error {
		f.mu.Lock()
		f.started[url]++
		delay := f.delay
		fail := f.failURL[url]
		f.mu.Unlock()

		if fail != nil {
			return fail
		}
		select {
		case <-time.After(delay):
			f.mu.Lock()
			f.done[url]++
			f.mu.Unlock()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (f *fakeWork) doneCount(url string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done[url]
}

func (f *fakeWork) startedCount(url string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started[url]
}

func TestCompletesAllFiles(t *testing.T) {
	f := newFakeWork()
	f.delay = time.Millisecond
	urls := []string{"/a/1.JPG", "/a/2.JPG", "/a/3.JPG", "/a/4.JPG"}
	tk := newTestTask(testStates, urls, f.run(), 1)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := tk.Wait(ctx); err != nil {
		t.Fatalf("wait: %v", err)
	}
	total := 0
	for _, u := range urls {
		total += f.doneCount(u)
	}
	if total != len(urls) {
		t.Fatalf("completed = %d, want %d", total, len(urls))
	}
	if tk.Status() != tsCompleted {
		t.Fatalf("status = %v, want completed", tk.Status())
	}
}

func TestConcurrency(t *testing.T) {
	f := newFakeWork()
	f.delay = 20 * time.Millisecond
	urls := []string{"/a/1.JPG", "/a/2.JPG", "/a/3.JPG", "/a/4.JPG"}
	tk := newTestTask(testStates, urls, f.run(), 2)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := tk.Wait(ctx); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if tk.Status() != tsCompleted {
		t.Fatalf("status = %v, want completed", tk.Status())
	}
}

func TestCancelInflight(t *testing.T) {
	f := newFakeWork()
	f.delay = 500 * time.Millisecond
	urls := []string{"/a/1.JPG", "/a/2.JPG"}
	tk := newTestTask(testStates, urls, f.run(), 1)

	deadline := time.Now().Add(time.Second)
	for !tkStarted(tk, urls[0]) {
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	tk.Cancel()

	select {
	case <-tk.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not finish in time")
	}
	if tk.Status() != tsCanceled {
		t.Fatalf("status = %v, want canceled", tk.Status())
	}
	if f.startedCount(urls[1]) != 0 {
		t.Fatalf("second file should not start, started %d", f.startedCount(urls[1]))
	}
}

func TestRemoveFromQueue(t *testing.T) {
	f := newFakeWork()
	f.delay = time.Millisecond
	urls := []string{"/a/1.JPG", "/a/2.JPG", "/a/3.JPG"}
	tk := newTestTask(testStates, urls, f.run(), 1)

	if !tk.RemoveFromQueue(urls[2]) {
		t.Fatal("expected removal")
	}
	if tk.RemoveFromQueue(urls[2]) {
		t.Fatal("second removal should be false")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tk.Wait(ctx)
	if f.doneCount(urls[2]) != 0 {
		t.Fatalf("removed file should not download, got %d", f.doneCount(urls[2]))
	}
	if f.doneCount(urls[0])+f.doneCount(urls[1]) != 2 {
		t.Fatalf("other files should complete, got %d/%d", f.doneCount(urls[0]), f.doneCount(urls[1]))
	}
}

func TestPauseResume(t *testing.T) {
	f := newFakeWork()
	f.delay = 5 * time.Millisecond
	var urls []string
	for i := 0; i < 12; i++ {
		urls = append(urls, string(rune('a'+i))+".JPG")
	}
	tk := newTestTask(testStates, urls, f.run(), 1)

	time.Sleep(30 * time.Millisecond)
	tk.Pause()
	time.Sleep(20 * time.Millisecond)
	if tk.Status() != tsPaused {
		t.Fatalf("status = %v, want paused", tk.Status())
	}
	tk.Resume()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := tk.Wait(ctx); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if tk.Status() != tsCompleted {
		t.Fatalf("status = %v, want completed", tk.Status())
	}
}

func TestPartialFailure(t *testing.T) {
	f := newFakeWork()
	f.delay = time.Millisecond
	f.failURL["/a/bad.JPG"] = errors.New("boom")
	urls := []string{"/a/ok1.JPG", "/a/bad.JPG", "/a/ok2.JPG"}
	tk := newTestTask(testStates, urls, f.run(), 1)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := tk.Wait(ctx); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if tk.Status() != tsFailed {
		t.Fatalf("status = %v, want failed", tk.Status())
	}
}

func TestEventsBroadcast(t *testing.T) {
	f := newFakeWork()
	f.delay = time.Millisecond
	urls := []string{"/a/1.JPG"}
	tk := newTestTask(testStates, urls, f.run(), 1)

	sub := tk.Bus().Subscribe()
	defer sub.Close()

	var got []testEvent
	deadline := time.After(2 * time.Second)
drain:
	for {
		select {
		case e := <-sub.Events():
			got = append(got, e)
			if e.task == tsCompleted {
				break drain
			}
		case <-deadline:
			break drain
		}
	}

	hasFileCompleted := false
	hasTaskCompleted := false
	for _, e := range got {
		if e.isTask && e.task == tsCompleted {
			hasTaskCompleted = true
		}
		if !e.isTask && e.file == fsCompleted {
			hasFileCompleted = true
		}
	}
	if !hasFileCompleted {
		t.Fatalf("missing file completed event: %+v", got)
	}
	if !hasTaskCompleted {
		t.Fatalf("missing task completed event: %+v", got)
	}
}

// tkStarted 通过事件订阅判断某文件是否已开始下载。
func tkStarted(tk *task.Task[ts, fs, testEvent], url string) bool {
	sub := tk.Bus().Subscribe()
	defer sub.Close()
	deadline := time.NewTimer(50 * time.Millisecond)
	defer deadline.Stop()
	for {
		select {
		case e := <-sub.Events():
			if !e.isTask && e.url == url && e.file == fsDownloading {
				return true
			}
		case <-deadline.C:
			return false
		}
	}
}
