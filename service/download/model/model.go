// Package model 定义下载任务持久化到 SQLite 的 bun 表模型。
package model

import (
	"time"

	"github.com/uptrace/bun"
)

// Task 对应 download_tasks 表：一次批量下载任务。
type Task struct {
	bun.BaseModel `bun:"table:download_tasks,alias:t"`

	ID         string     `bun:"id,pk"`
	Status     int32      `bun:"status,notnull"`      // downloadv1.TaskStatus
	TotalFiles int        `bun:"total_files,notnull"` // 文件总数
	Completed  int        `bun:"completed,notnull"`
	Failed     int        `bun:"failed,notnull"`
	Note       string     `bun:"note"` // 异常说明，如进程退出导致的中断
	CreatedAt  time.Time  `bun:"created_at,notnull"`
	UpdatedAt  time.Time  `bun:"updated_at,notnull"`
	FinishedAt *time.Time `bun:"finished_at"`
}

// File 对应 download_files 表：任务内的单个文件。
// (task_id, url) 唯一，id 自增用于保持入队顺序。
type File struct {
	bun.BaseModel `bun:"table:download_files,alias:f"`

	ID           int64     `bun:"id,pk,autoincrement"`
	TaskID       string    `bun:"task_id,notnull,unique:task_url"`
	URL          string    `bun:"url,notnull,unique:task_url"`
	Folder       string    `bun:"folder,notnull"`
	Name         string    `bun:"name,notnull"`
	Status       int32     `bun:"status,notnull"` // downloadv1.FileStatus
	ProgressPct  int32     `bun:"progress_pct,notnull"`
	Downloaded   int64     `bun:"downloaded_bytes,notnull"`
	TotalBytes   int64     `bun:"total_bytes,notnull"` // -1 表示未知
	ErrorMessage string    `bun:"error_message"`
	UpdatedAt    time.Time `bun:"updated_at,notnull"`
}
