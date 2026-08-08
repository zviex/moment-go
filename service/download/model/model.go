// Package model 只提供任务在持久化存储（如 SQLite）中的数据模型，
// 不包含任何数据库操作。模型使用 sqlite 友好的标签；若要接入 GORM，
// 只需在结构体上补充 gorm 标签即可。
package model

import "time"

// Task 描述一次批量下载任务在数据库中的持久化形态。
type Task struct {
	// ID 任务唯一标识。
	ID string `sqlite:"task_id;type:text;primary key"`
	// Name 可选任务名。
	Name string `sqlite:"name;type:text"`
	// TotalFiles 本次任务的文件总数。
	TotalFiles int `sqlite:"total_files;type:integer"`
	// Completed 已完成数。
	Completed int `sqlite:"completed;type:integer"`
	// Failed 失败数。
	Failed int `sqlite:"failed;type:integer"`
	// Canceled 被移除/取消数。
	Canceled int `sqlite:"canceled;type:integer"`
	// Status TaskStatus 枚举值。
	Status int `sqlite:"status;type:integer"`
	// Concurrency worker 并发数。
	Concurrency int `sqlite:"concurrency;type:integer"`
	// CreatedAt 创建时间。
	CreatedAt time.Time `sqlite:"created_at;type:datetime"`
	// UpdatedAt 更新时间。
	UpdatedAt time.Time `sqlite:"updated_at;type:datetime"`
	// FinishedAt 任务结束时间（未结束为 nil）。
	FinishedAt *time.Time `sqlite:"finished_at;type:datetime;null"`
}

// File 描述任务中单个文件的持久化形态。
// 记录每个 url 在任务内的下载状态，重启后可据此恢复。
type File struct {
	ID           int64     `sqlite:"id;type:integer;primary key;auto increment"`
	TaskID       string    `sqlite:"task_id;type:text;index"` // 外键指向 Task.ID
	URL          string    `sqlite:"url;type:text"`           // 远端/相机文件引用
	Folder       string    `sqlite:"folder;type:text"`        // 解析后的相机目录
	Name         string    `sqlite:"name;type:text"`          // 解析后的文件名
	Status       int       `sqlite:"status;type:integer"`     // FileStatus 枚举值
	ProgressPct  int32     `sqlite:"progress_pct;type:integer"`
	Downloaded   int64     `sqlite:"downloaded_bytes;type:integer"`
	TotalBytes   int64     `sqlite:"total_bytes;type:integer;null"` // -1 表示未知
	ErrorMessage string    `sqlite:"error_message;type:text;null"`
	CreatedAt    time.Time `sqlite:"created_at;type:datetime"`
	UpdatedAt    time.Time `sqlite:"updated_at;type:datetime"`
}
