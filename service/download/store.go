package download

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ci.zevix.uk/moment-go/service/download/model"
	downloadv1 "ci.zevix.uk/moment/moment-protogo/download/v1"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// ErrTaskNotFound 表示历史库中不存在该任务。
var ErrTaskNotFound = errors.New("download: task not found in history")

// Store 把下载任务与文件的状态持久化到 SQLite（bun ORM）。
// 所有方法在 s 为 nil 时都是空操作，便于未启用持久化时直接调用。
type Store struct {
	db *bun.DB
}

// OpenStore 打开（必要时创建）SQLite 数据库，建表，并把上次进程遗留的
// 未结束任务标记为失败（进程已退出，它们不可能再继续）。
func OpenStore(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = filepath.Join("moment", "data", "download.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("download store: mkdir: %w", err)
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		return nil, fmt.Errorf("download store: open: %w", err)
	}
	// SQLite 单写者：限制为单连接，避免 "database is locked"。
	sqldb.SetMaxOpenConns(1)
	db := bun.NewDB(sqldb, sqlitedialect.New())

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.markInterrupted(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库。
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	models := []any{(*model.Task)(nil), (*model.File)(nil)}
	for _, m := range models {
		if _, err := s.db.NewCreateTable().Model(m).IfNotExists().Exec(ctx); err != nil {
			return fmt.Errorf("download store: create table: %w", err)
		}
	}
	if _, err := s.db.NewCreateIndex().Model((*model.Task)(nil)).
		Index("idx_download_tasks_created_at").Column("created_at").IfNotExists().Exec(ctx); err != nil {
		return fmt.Errorf("download store: create index: %w", err)
	}
	return nil
}

// markInterrupted 把上次进程遗留的运行中/暂停中的任务及其未结束文件标记为失败。
func (s *Store) markInterrupted(ctx context.Context) error {
	now := time.Now()
	const note = "interrupted: process exited before the task finished"
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model((*model.Task)(nil)).
			Set("status = ?", int32(downloadv1.TaskStatus_TASK_STATUS_FAILED)).
			Set("note = ?", note).
			Set("finished_at = ?", now).
			Set("updated_at = ?", now).
			Where("status IN (?)", bun.In([]int32{
				int32(downloadv1.TaskStatus_TASK_STATUS_RUNNING),
				int32(downloadv1.TaskStatus_TASK_STATUS_PAUSED),
			})).
			Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Model((*model.File)(nil)).
			Set("status = ?", int32(downloadv1.FileStatus_FILE_STATUS_FAILED)).
			Set("error_message = ?", note).
			Set("updated_at = ?", now).
			Where("status IN (?)", bun.In([]int32{
				int32(downloadv1.FileStatus_FILE_STATUS_PENDING),
				int32(downloadv1.FileStatus_FILE_STATUS_DOWNLOADING),
				int32(downloadv1.FileStatus_FILE_STATUS_PAUSED),
			})).
			Exec(ctx)
		return err
	})
}

// CreateTask 登记一次新任务及其全部文件（初始为 PENDING）。
// 重复 URL 只保留第一条，与队列去重语义保持一致。
func (s *Store) CreateTask(ctx context.Context, id string, urls []string) error {
	if s == nil {
		return nil
	}
	now := time.Now()
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		task := &model.Task{
			ID:         id,
			Status:     int32(downloadv1.TaskStatus_TASK_STATUS_RUNNING),
			TotalFiles: len(urls),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if _, err := tx.NewInsert().Model(task).Exec(ctx); err != nil {
			return err
		}
		files := make([]model.File, 0, len(urls))
		for _, u := range urls {
			ref := parseFileRef(u)
			files = append(files, model.File{
				TaskID:     id,
				URL:        u,
				Folder:     ref.folder,
				Name:       ref.name,
				Status:     int32(downloadv1.FileStatus_FILE_STATUS_PENDING),
				TotalBytes: -1,
				UpdatedAt:  now,
			})
		}
		if len(files) == 0 {
			return nil
		}
		_, err := tx.NewInsert().Model(&files).On("CONFLICT (task_id, url) DO NOTHING").Exec(ctx)
		return err
	})
}

// RecordFile 更新单个文件的状态与进度。
func (s *Store) RecordFile(ctx context.Context, taskID, url string, status downloadv1.FileStatus,
	progressPct int32, downloaded, total int64, errMsg string) error {
	if s == nil {
		return nil
	}
	_, err := s.db.NewUpdate().Model((*model.File)(nil)).
		Set("status = ?", int32(status)).
		Set("progress_pct = ?", progressPct).
		Set("downloaded_bytes = ?", downloaded).
		Set("total_bytes = ?", total).
		Set("error_message = ?", errMsg).
		Set("updated_at = ?", time.Now()).
		Where("task_id = ? AND url = ?", taskID, url).
		Exec(ctx)
	return err
}

// RecordTask 更新整体任务的状态与计数；进入终态时写入结束时间。
func (s *Store) RecordTask(ctx context.Context, taskID string, status downloadv1.TaskStatus,
	completed, failed int) error {
	if s == nil {
		return nil
	}
	now := time.Now()
	q := s.db.NewUpdate().Model((*model.Task)(nil)).
		Set("status = ?", int32(status)).
		Set("completed = ?", completed).
		Set("failed = ?", failed).
		Set("updated_at = ?", now).
		Where("id = ?", taskID)
	if isTerminal(status) {
		q = q.Set("finished_at = ?", now)
	}
	_, err := q.Exec(ctx)
	return err
}

func isTerminal(st downloadv1.TaskStatus) bool {
	switch st {
	case downloadv1.TaskStatus_TASK_STATUS_COMPLETED,
		downloadv1.TaskStatus_TASK_STATUS_FAILED,
		downloadv1.TaskStatus_TASK_STATUS_CANCELED:
		return true
	}
	return false
}

// TaskRecord 是历史任务的读取结果，包含其全部文件。
type TaskRecord struct {
	Task  model.Task
	Files []model.File
}

// ListTasks 按创建时间倒序返回历史任务（不含文件明细）。
func (s *Store) ListTasks(ctx context.Context, limit, offset int) ([]model.Task, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	var tasks []model.Task
	err := s.db.NewSelect().Model(&tasks).
		OrderExpr("created_at DESC, id DESC").
		Limit(int64(limit)).Offset(int64(offset)).
		Scan(ctx)
	return tasks, err
}

// GetTask 返回一个历史任务及其文件（按入队顺序）。
func (s *Store) GetTask(ctx context.Context, id string) (*TaskRecord, error) {
	if s == nil {
		return nil, ErrTaskNotFound
	}
	task := new(model.Task)
	if err := s.db.NewSelect().Model(task).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	var files []model.File
	if err := s.db.NewSelect().Model(&files).
		Where("task_id = ?", id).
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	return &TaskRecord{Task: *task, Files: files}, nil
}
