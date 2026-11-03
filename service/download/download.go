package download

import (
	"context"
	"errors"

	downloadv1 "ci.zevix.uk/moment/moment-protogo/download/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ downloadv1.DownloadServiceServer = &Service{}

// Service 实现 download.v1.DownloadService。
type Service struct {
	downloadv1.UnimplementedDownloadServiceServer

	manager *Manager
}

func New(manager *Manager) *Service {
	return &Service{manager: manager}
}

// CreateTask 接收远端传来的待下载 url 列表，创建一次批量下载任务。
func (s *Service) CreateTask(ctx context.Context, req *downloadv1.CreateTaskRequest) (*downloadv1.CreateTaskResponse, error) {
	if req == nil || len(req.GetUrls()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "download: empty url list")
	}

	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}

	t, err := s.manager.create(ctx, req.GetUrls())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "download: create task: %v", err)
	}

	return &downloadv1.CreateTaskResponse{
		TaskId:     t.taskID(),
		TotalFiles: int32(len(req.GetUrls())),
	}, nil
}

// MonitorTask 是流式接口，持续推送单个文件与整体任务的状态事件。
func (s *Service) MonitorTask(req *downloadv1.MonitorTaskRequest, stream grpc.ServerStreamingServer[downloadv1.MonitorTaskResponse]) error {
	if req == nil || req.GetTaskId() == "" {
		return status.Error(codes.InvalidArgument, "download: missing task id")
	}

	t, ok := s.manager.get(req.GetTaskId())
	if !ok {
		// 内存中已回收的任务，从历史库回放其最终状态。
		rec, err := s.manager.history(stream.Context(), req.GetTaskId())
		if err != nil {
			if errors.Is(err, ErrTaskNotFound) {
				return status.Error(codes.NotFound, "download: task not found")
			}
			return status.Errorf(codes.Internal, "download: load history: %v", err)
		}
		for _, ev := range historyEvents(rec) {
			if err := stream.Send(ev); err != nil {
				return err
			}
		}
		return nil
	}

	sub := t.SubscribeEvents()
	defer sub.Close()

	// 先回放一次当前整体状态，避免客户端订阅晚于开头的 RUNNING 事件。
	t.Snapshot()

	ctx := stream.Context()
	// MonitorTask 是长生命周期流。gRPC 的 GracefulStop 不会取消在途流的
	// ctx，如果只依赖 stream.Context()，进程收到 SIGINT 后会一直等客户端
	// 关闭流。这里额外监听服务根 ctx（随进程信号取消），保证优雅关闭能
	// 正常结束。
	rootDone := s.manager.root(ctx).Done()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-rootDone:
			return nil
		case <-sub.Done():
			return nil
		case ev, ok := <-sub.Events():
			if !ok {
				return nil
			}

			if err := stream.Send(ev); err != nil {
				return err
			}
		}
	}
}

// ControlTask 控制任务或单独文件：暂停/恢复/取消/移除队列。
func (s *Service) ControlTask(ctx context.Context, req *downloadv1.ControlTaskRequest) (*downloadv1.ControlTaskResponse, error) {
	if req == nil || req.GetTaskId() == "" {
		return nil, status.Error(codes.InvalidArgument, "download: missing task id")
	}

	t, ok := s.manager.get(req.GetTaskId())
	if !ok {
		return nil, status.Error(codes.NotFound, "download: task not found")
	}

	// 指定 url 时针对单个文件（仅支持取消/移除队列）。
	if req.Url != nil {
		if req.GetAction() != downloadv1.ControlAction_CONTROL_ACTION_CANCEL {
			return nil, status.Error(codes.Unimplemented, "download: single-file control only supports cancel")
		}

		if t.RemoveFromQueue(req.GetUrl()) {
			return okResponse("file removed from download queue"), nil
		}

		return nil, status.Error(codes.NotFound, "download: file not in queue or in-flight")
	}

	switch req.GetAction() {
	case downloadv1.ControlAction_CONTROL_ACTION_PAUSE:
		t.Pause()
	case downloadv1.ControlAction_CONTROL_ACTION_CANCEL:
		t.Cancel()
	default:
		return nil, status.Error(codes.InvalidArgument, "download: unknown control action")
	}

	return okResponse("ok"), nil
}

func okResponse(msg string) *downloadv1.ControlTaskResponse {
	return &downloadv1.ControlTaskResponse{Success: true, Message: msg}
}
