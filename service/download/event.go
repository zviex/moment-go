package download

import (
	"time"

	gktask "ci.zevix.uk/moment-go/pkg/task"
	downloadv1 "ci.zevix.uk/moment/moment-protogo/download/v1"
)

// taskStatus download 模块使用的整体任务状态枚举。
type taskStatus = downloadv1.TaskStatus

// fileStatus download 模块使用的单文件状态枚举。
type fileStatus = downloadv1.FileStatus

// TaskState download 使用的整体任务状态语义映射。
var taskState = gktask.States[taskStatus, fileStatus]{
	TaskRunning:   downloadv1.TaskStatus_TASK_STATUS_RUNNING,
	TaskPaused:    downloadv1.TaskStatus_TASK_STATUS_PAUSED,
	TaskCanceled:  downloadv1.TaskStatus_TASK_STATUS_CANCELED,
	TaskCompleted: downloadv1.TaskStatus_TASK_STATUS_COMPLETED,
	TaskFailed:    downloadv1.TaskStatus_TASK_STATUS_FAILED,

	FileDownloading: downloadv1.FileStatus_FILE_STATUS_DOWNLOADING,
	FileCompleted:   downloadv1.FileStatus_FILE_STATUS_COMPLETED,
	FileFailed:      downloadv1.FileStatus_FILE_STATUS_FAILED,
	FileCanceled:    downloadv1.FileStatus_FILE_STATUS_CANCELED,
}

// buildMonitorEvent 是 download 模块"自己的特殊逻辑"：把泛型 task 内部事件
// 转换为本模块对外广播的 *downloadv1.MonitorTaskResponse。
func buildMonitorEvent(taskID string) func(gktask.Event[taskStatus, fileStatus]) *downloadv1.MonitorTaskResponse {
	return func(ev gktask.Event[taskStatus, fileStatus]) *downloadv1.MonitorTaskResponse {
		resp := &downloadv1.MonitorTaskResponse{
			TaskId:    taskID,
			Timestamp: time.Now().UnixMilli(),
		}
		switch {
		case ev.IsFile():
			resp.Type = downloadv1.EventType_EVENT_TYPE_FILE_UPDATE
			resp.FileEvent = &downloadv1.FileEvent{
				Url:             ev.URL,
				Status:          ev.FileStatus,
				ProgressPercent: ev.Progress.Percent(),
				DownloadedBytes: ev.Progress.Downloaded,
				TotalBytes:      ev.Progress.Total,
				ErrorMessage:    ev.Error,
			}
		case ev.IsTask():
			resp.Type = downloadv1.EventType_EVENT_TYPE_TASK_UPDATE
			resp.TaskStateEvent = &downloadv1.TaskStateEvent{
				Status:         ev.TaskStatus,
				CompletedFiles: int32(ev.Counts.Completed),
				FailedFiles:    int32(ev.Counts.Failed),
				TotalFiles:     int32(ev.Counts.Total),
			}
		}
		return resp
	}
}
