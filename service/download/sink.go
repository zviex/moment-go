package download

import (
	"context"
	"errors"
	"fmt"
	"io"

	"ci.zevix.uk/moment-go/adapter/ports"
	"ci.zevix.uk/moment-go/pkg/task"
	"ci.zevix.uk/moment-go/storage"
)

var (
	ErrNoCamera  = errors.New("download: camera session unavailable")
	ErrNoStorage = errors.New("download: storage unavailable")
)

func NewCameraDownloader(reader ports.FileReader, store storage.Storage) ports.Downloader {
	return &cameraDownloader{reader: reader, storage: store}
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
	reader  ports.FileReader
	storage storage.Storage
}

// Download 解析 url 为相机内 fileRef，把文件内容写入存储，
// 并在开始、1/4 阈值、完成时触发 onProgress。
func (d *cameraDownloader) Download(ctx context.Context, url string, onProgress func(task.Progress)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.reader == nil {
		return ErrNoCamera
	}
	if d.storage == nil {
		return ErrNoStorage
	}

	if onProgress == nil {
		onProgress = func(task.Progress) {}
	}

	ref := parseFileRef(url)

	total, err := d.fileSize(ctx, ref)
	if err != nil {
		// 大小未知时 total 为 -1，进度仍可上报完成事件。
		total = -1
	}

	watcher := task.NewQuarterWatcher(task.Progress{URL: url, Downloaded: 0, Total: total}, onProgress)

	// 写入存储；writer 计数用于进度计算。
	var counter countWriter

	writeErr := storage.Write(ctx, d.storage, ref.localName(), func(w io.Writer) error {
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

	if err := d.reader.DownloadTo(ctx, ref.folder, ref.name, w); err != nil {
		return err
	}
	// 若流式实现逐步写入了字节，这里也能逐阈值推进。
	watcher.Update(task.Progress{URL: url, Downloaded: total, Total: total})
	return nil
}

// fileSize 尝试获取文件大小；无法获取时返回 -1。
func (d *cameraDownloader) fileSize(ctx context.Context, ref fileRef) (int64, error) {
	info, err := d.reader.GetFileInfo(ctx, ref.folder, ref.name)
	if err != nil {
		return -1, err
	}
	return info.GetSize(), nil
}
