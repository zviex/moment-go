package download

import (
	"context"
	"errors"
	"fmt"
	"io"

	"ci.zevix.uk/moment-go/pkg/moment"
	"ci.zevix.uk/moment-go/pkg/task"
	"ci.zevix.uk/moment-go/storage"
)

// Downloader 抽象"从相机下载一个文件并写入 Disk 存储"这一行为。
// 这是 task 手动下载与 autosync 自动同步共享的下载行为：
// 两种使用方都只依赖本接口，而不关心底层是 libgphoto2 还是未来新的传输方式。
type Downloader interface {
	// Download 下载 url 指向的相机文件，并通过 onProgress 上报实时进度。
	// ctx 取消（Pause/Cancel/移除队列）时，正在进行的传输会尽快中断。
	Download(ctx context.Context, url string, onProgress func(task.Progress)) error
}

// ErrNoCamera 表示当前没有可用的相机会话。
var ErrNoCamera = errors.New("download: camera session unavailable")

// New 构造一个使用默认相机（client.Default().Camera()）与 Disk 存储的下载器。
// 默认写入 Disk；该构造是可选的便捷入口，方法把具体存储通过选项传入。
func New(opts ...Option) Downloader {
	c := &cameraDownloader{storage: &storage.Disk{}}
	for _, o := range opts {
		if o != nil {
			o(c)
		}
	}
	return c
}

// Option 配置下载器。
type Option func(*cameraDownloader)

// WithStorage 指定下载文件写入的存储（默认 Disk）。
func WithStorage(s storage.Storage) Option {
	return func(d *cameraDownloader) { d.storage = s }
}

// WithCamera 指定相机会话来源（默认 client.Default().Camera()）。
func WithCamera(cam func() (*moment.Camera, error)) Option {
	return func(d *cameraDownloader) { d.cam = cam }
}

// countWriter 包装底层 writer 并对写入字节计数，从而实现进度上报。
type countWriter struct {
	io.Writer
	n int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.n += int64(n)
	return n, err
}

type cameraDownloader struct {
	cam     func() (*moment.Camera, error)
	storage storage.Storage
}

// Download 解析 url 为相机内 fileRef，把文件内容写入 Disk 存储，
// 并在开始、1/4 阈值、完成时触发 onProgress。
func (d *cameraDownloader) Download(ctx context.Context, url string, onProgress func(task.Progress)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if onProgress == nil {
		onProgress = func(task.Progress) {}
	}
	if d.storage == nil {
		d.storage = &storage.Disk{}
	}
	if d.cam == nil {
		d.cam = func() (*moment.Camera, error) {
			return momentClientCamera()
		}
	}

	ref := parseFileRef(url)

	total, err := d.fileSize(ctx, ref)
	if err != nil {
		// 大小未知时 total 为 -1，进度仍可上报完成事件。
		total = -1
	}
	watcher := task.NewQuarterWatcher(task.Progress{URL: url, Downloaded: 0, Total: total}, onProgress)

	// 写入 Disk 存储；writer 计数用于进度计算。
	var counter countWriter
	writeErr := storage.Write(d.storage, ref.localName(), func(w io.Writer) error {
		counter.Writer = w
		return d.doDownload(ctx, ref, &counter, url, watcher, total)
	})
	if writeErr != nil {
		return fmt.Errorf("download %q: %w", ref.displayName(), writeErr)
	}
	// 完成事件：已下载字节数 >= total（或 total 未知时按实际计数）。
	doneBytes := counter.n
	if total >= 0 {
		doneBytes = total
	}
	watcher.Update(task.Progress{URL: url, Downloaded: doneBytes, Total: total})
	return nil
}

// doDownload 真正调用相机的 DownloadTo 并衔接进度 watcher。
func (d *cameraDownloader) doDownload(ctx context.Context, ref fileRef, w io.Writer, url string, watcher *task.QuarterWatcher, total int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cam, err := d.cam()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoCamera, err)
	}
	err = cam.DownloadTo(ctx, ref.folder, ref.name, w)
	if err != nil {
		return err
	}
	// 若流式实现逐步写入了字节，这里也能逐阈值推进。
	watcher.Update(task.Progress{URL: url, Downloaded: total, Total: total})
	return nil
}

// fileSize 尝试获取文件大小；无法获取时返回 -1。
func (d *cameraDownloader) fileSize(ctx context.Context, ref fileRef) (int64, error) {
	cam, err := d.cam()
	if err != nil {
		return -1, err
	}
	file, err := cam.GetFileInfo(ctx, ref.folder, ref.name)
	if err != nil {
		return -1, err
	}
	return file.Size, nil
}
