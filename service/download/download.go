package download

import (
	"context"

	downloadv1 "ci.zevix.uk/moment-go/protogo/download/v1"
	"google.golang.org/grpc"
)

type download struct {
	downloadv1.UnimplementedDownloadServiceServer
}

func (d *download) ControlTask(context.Context, *downloadv1.ControlTaskRequest) (*downloadv1.ControlTaskResponse, error) {
	panic("unimplemented")
}

func (d *download) CreateTask(context.Context, *downloadv1.CreateTaskRequest) (*downloadv1.CreateTaskResponse, error) {
	panic("unimplemented")
}

func (d *download) MonitorTask(*downloadv1.MonitorTaskRequest, grpc.ServerStreamingServer[downloadv1.MonitorTaskResponse]) error {
	panic("unimplemented")
}
