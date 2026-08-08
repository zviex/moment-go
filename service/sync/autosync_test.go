package sync

import (
	"context"
	stdsync "sync"
	"testing"
	"time"

	gktask "ci.zevix.uk/moment-go/pkg/task"
)

// captureStub 是模拟的拍摄事件源。
type captureStub struct {
	urls []string
}

// Events 按序吐出每个 url 对应的拍摄事件。
func (c *captureStub) Events(ctx context.Context) (<-chan CaptureEvent, error) {
	ch := make(chan CaptureEvent)
	go func() {
		defer close(ch)
		for _, u := range c.urls {
			select {
			case ch <- CaptureEvent{URL: u, At: time.Now()}:
			case <-ctx.Done():
				return
			}
		}
		// 事件发完后保持通道打开，等待 ctx 取消，贴近真实事件流。
		<-ctx.Done()
	}()
	return ch, nil
}

// recordDownloader 记录被下发的 url，作为共享 Downloader 的测试替身。
type recordDownloader struct {
	mu   stdsync.Mutex
	urls []string
}

func (r *recordDownloader) Download(_ context.Context, url string, _ func(gktask.Progress)) error {
	r.mu.Lock()
	r.urls = append(r.urls, url)
	r.mu.Unlock()
	return nil
}

func (r *recordDownloader) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.urls))
	copy(out, r.urls)
	return out
}

func TestRecorderDownloadsCaptureEvents(t *testing.T) {
	src := &captureStub{urls: []string{"/DCIM/IMG_1.JPG", "/DCIM/IMG_2.JPG"}}
	dl := &recordDownloader{}

	r := &Recorder{downloader: dl, source: src}
	if err := r.Enable(); err != nil {
		t.Fatalf("enable: %v", err)
	}
	defer r.Disable()

	deadline := time.Now().Add(2 * time.Second)
	for len(dl.list()) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("expected 2 downloads, got %v", dl.list())
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := dl.list()
	if len(got) != 2 || got[0] != "/DCIM/IMG_1.JPG" || got[1] != "/DCIM/IMG_2.JPG" {
		t.Fatalf("downloaded %v", got)
	}
}

func TestRecorderDisabledDoesNotConsumeSource(t *testing.T) {
	src := &captureStub{urls: []string{"/DCIM/IMG_1.JPG"}}
	dl := &recordDownloader{}
	r := &Recorder{downloader: dl, source: src}

	time.Sleep(50 * time.Millisecond)
	if r.Enabled() {
		t.Fatal("should be disabled before Enable")
	}
	if got := dl.list(); len(got) != 0 {
		t.Fatalf("no download expected before enable, got %v", got)
	}
}
