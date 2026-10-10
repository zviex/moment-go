package download

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	downloadv1 "ci.zevix.uk/moment/moment-protogo/download/v1"
)

func TestStoreRoundTripAndHistory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "download.db")

	s, err := OpenStore(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	urls := []string{"/store/100CANON/IMG_0001.JPG", "/store/100CANON/IMG_0002.JPG"}
	if err := s.CreateTask(ctx, "t1", urls); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.RecordFile(ctx, "t1", urls[0], downloadv1.FileStatus_FILE_STATUS_COMPLETED, 100, 10, 10, ""); err != nil {
		t.Fatalf("record file: %v", err)
	}
	if err := s.RecordFile(ctx, "t1", urls[1], downloadv1.FileStatus_FILE_STATUS_FAILED, 0, 0, -1, "boom"); err != nil {
		t.Fatalf("record file: %v", err)
	}
	if err := s.RecordTask(ctx, "t1", downloadv1.TaskStatus_TASK_STATUS_FAILED, 1, 1); err != nil {
		t.Fatalf("record task: %v", err)
	}

	rec, err := s.GetTask(ctx, "t1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rec.Task.TotalFiles != 2 || rec.Task.Completed != 1 || rec.Task.Failed != 1 {
		t.Fatalf("task counts wrong: %+v", rec.Task)
	}
	if rec.Task.FinishedAt == nil {
		t.Fatal("terminal task should have finished_at")
	}
	if len(rec.Files) != 2 || rec.Files[0].URL != urls[0] || rec.Files[0].Folder != "/store/100CANON" || rec.Files[0].Name != "IMG_0001.JPG" {
		t.Fatalf("files wrong / not in order: %+v", rec.Files)
	}
	if rec.Files[1].ErrorMessage != "boom" {
		t.Fatalf("error message not persisted: %+v", rec.Files[1])
	}

	list, err := s.ListTasks(ctx, 10, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}

	evs := historyEvents(rec)
	if len(evs) != 3 || evs[2].GetTaskStateEvent().GetStatus() != downloadv1.TaskStatus_TASK_STATUS_FAILED {
		t.Fatalf("history events wrong: %+v", evs)
	}

	if _, err := s.GetTask(ctx, "missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("want ErrTaskNotFound, got %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// 模拟进程重启：遗留的运行中任务应被标记为失败。
	if err := s.CreateTask(ctx, "t2", []string{"/store/a.jpg"}); err == nil {
		t.Fatal("closed store should fail")
	}
	s2, err := OpenStore(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if err := s2.CreateTask(ctx, "t2", []string{"/store/a.jpg"}); err != nil {
		t.Fatal(err)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
	s4, err := OpenStore(ctx, path)
	if err != nil {
		t.Fatalf("reopen2: %v", err)
	}
	defer s4.Close()
	rec2, err := s4.GetTask(ctx, "t2")
	if err != nil {
		t.Fatal(err)
	}
	if rec2.Task.Status != int32(downloadv1.TaskStatus_TASK_STATUS_FAILED) || rec2.Task.Note == "" {
		t.Fatalf("interrupted task not marked: %+v", rec2.Task)
	}
	if rec2.Files[0].Status != int32(downloadv1.FileStatus_FILE_STATUS_FAILED) {
		t.Fatalf("interrupted file not marked: %+v", rec2.Files[0])
	}
}

func TestNilStoreIsNoop(t *testing.T) {
	var s *Store
	ctx := context.Background()
	if err := s.CreateTask(ctx, "x", []string{"/a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordTask(ctx, "x", downloadv1.TaskStatus_TASK_STATUS_RUNNING, 0, 0); err != nil {
		t.Fatal(err)
	}
}
