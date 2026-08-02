// Package usecase 是整洁架构的应用层：编排领域实体与端口，实现用例，
// 围绕生命周期状态机组织连接、浏览、下载与信息解析等操作。
package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"ci.zevix.uk/moment-go/internal/domain"
	"ci.zevix.uk/moment-go/internal/logging"
)

// maxWalkDepth 防止相机返回异常深度的目录树时无限递归。
const maxWalkDepth = 32

// CameraApp 组合网关与生命周期，向 delivery 层暴露用例。
type CameraApp struct {
	gateway   domain.Gateway
	session   domain.CameraSession
	lifecycle *Lifecycle
	logger    logging.Logger
}

func NewCameraApp(gateway domain.Gateway, logger logging.Logger) *CameraApp {
	if logger == nil {
		logger = logging.Nop{}
	}
	return &CameraApp{
		gateway:   gateway,
		lifecycle: NewLifecycle(),
		logger:    logger,
	}
}

// State 返回当前生命周期状态。
func (a *CameraApp) State() domain.State { return a.lifecycle.State() }

// LastError 返回最近一次失败原因。
func (a *CameraApp) LastError() error { return a.lifecycle.Error() }

// IsConnected 是否处于已连接状态。
func (a *CameraApp) IsConnected() bool { return a.State() == domain.StateConnected }

// Detect 探测可用相机（USB 自动检测）。
func (a *CameraApp) Detect(ctx context.Context) ([]domain.DetectedCamera, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.gateway.Detect(ctx)
}

// Connect 建立相机连接。成功后将生命周期切换到 Connected。
func (a *CameraApp) Connect(ctx context.Context, opts domain.ConnectOptions) (domain.CameraSession, error) {
	if a.IsConnected() {
		return nil, domain.ErrAlreadyConnected
	}
	if err := a.lifecycle.transition(domain.StateDisconnected, domain.StateConnecting); err != nil {
		return nil, err
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	a.logger.Infof("connecting camera: type=%v model=%q host=%q port=%d", opts.Type, opts.Model, opts.Host, opts.Port)
	sess, err := a.gateway.Connect(ctx, opts)
	if err != nil {
		a.lifecycle.fail(err)
		a.logger.Errorf("connect failed: %v", err)
		return nil, err
	}
	a.session = sess
	if err := a.lifecycle.transition(domain.StateConnecting, domain.StateConnected); err != nil {
		_ = sess.Close(ctx)
		a.session = nil
		return nil, err
	}
	a.logger.Infof("connected: model=%q driver=%q port=%q", sess.Model(), sess.Driver(), sess.PortPath())
	return sess, nil
}

// Disconnect 断开连接并释放会话资源。
func (a *CameraApp) Disconnect(ctx context.Context) error {
	switch a.State() {
	case domain.StateDisconnected:
		return nil
	case domain.StateConnecting, domain.StateDisconnecting:
		return domain.ErrInvalidState
	}
	if err := a.lifecycle.transition(domain.StateConnected, domain.StateDisconnecting); err != nil {
		return err
	}
	var closeErr error
	if a.session != nil {
		closeErr = a.session.Close(ctx)
		a.session = nil
	}
	_ = a.lifecycle.transition(domain.StateDisconnecting, domain.StateDisconnected)
	if closeErr != nil {
		a.logger.Warnf("disconnect: %v", closeErr)
	}
	a.logger.Infof("disconnected")
	return closeErr
}

// Session 返回当前已连接的会话；未连接时返回 ErrNotConnected。
func (a *CameraApp) Session() (domain.CameraSession, error) {
	if a.State() != domain.StateConnected || a.session == nil {
		return nil, domain.ErrNotConnected
	}
	return a.session, nil
}

// ListFolders 列出指定目录下的子目录。
func (a *CameraApp) ListFolders(ctx context.Context, folder string) ([]domain.Folder, error) {
	sess, err := a.Session()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sess.ListFolders(ctx, folder)
}

// ListFiles 列出指定目录下的文件（含文件信息）。
func (a *CameraApp) ListFiles(ctx context.Context, folder string) ([]domain.File, error) {
	sess, err := a.Session()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sess.ListFiles(ctx, folder)
}

// ListTree 递归遍历相机全部目录与文件，用于"预览所有相机内容"。
func (a *CameraApp) ListTree(ctx context.Context) (folders []domain.Folder, files []domain.File, err error) {
	sess, err := a.Session()
	if err != nil {
		return nil, nil, err
	}
	folders = append(folders, domain.Folder{Name: "/", Path: "/"})
	if err := a.walk(ctx, sess, "/", 0, &folders, &files); err != nil {
		return nil, nil, err
	}
	return folders, files, nil
}

// walk 深度优先遍历目录树。
func (a *CameraApp) walk(ctx context.Context, sess domain.CameraSession, path string, depth int,
	folders *[]domain.Folder, files *[]domain.File) error {
	if depth > maxWalkDepth {
		return fmt.Errorf("directory tree too deep at %q", path)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sub, err := sess.ListFolders(ctx, path)
	if err != nil {
		// 某些相机对根目录下的特殊存储节点不支持列目录，跳过并记录。
		a.logger.Warnf("list folders %q: %v", path, err)
	} else {
		for _, f := range sub {
			*folders = append(*folders, f)
			if err := a.walk(ctx, sess, f.Path, depth+1, folders, files); err != nil {
				return err
			}
		}
	}
	fs, err := sess.ListFiles(ctx, path)
	if err != nil {
		return err
	}
	*files = append(*files, fs...)
	return nil
}

// GetFileInfo 查询单个文件信息。
func (a *CameraApp) GetFileInfo(ctx context.Context, folder, name string) (domain.File, error) {
	sess, err := a.Session()
	if err != nil {
		return domain.File{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.File{}, err
	}
	return sess.GetFileInfo(ctx, folder, name)
}

// Download 下载文件完整内容到内存。
func (a *CameraApp) Download(ctx context.Context, folder, name string) ([]byte, error) {
	sess, err := a.Session()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sess.Download(ctx, folder, name)
}

// DownloadTo 下载文件并写入 w。
func (a *CameraApp) DownloadTo(ctx context.Context, folder, name string, w io.Writer) error {
	sess, err := a.Session()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return sess.DownloadTo(ctx, folder, name, w)
}

// GetInfo 聚合相机信息（摘要 / 说明 / 存储 / 配置树）。
func (a *CameraApp) GetInfo(ctx context.Context) (domain.CameraInfo, error) {
	sess, err := a.Session()
	if err != nil {
		return domain.CameraInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.CameraInfo{}, err
	}
	info, err := sess.GetInfo(ctx)
	if err != nil {
		return domain.CameraInfo{}, err
	}
	return info, nil
}

// Close 释放应用层持有的资源（网关）。
func (a *CameraApp) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = a.Disconnect(ctx)
	if a.gateway != nil {
		return a.gateway.Close()
	}
	return nil
}

// IsCancelled 判断错误是否为取消类错误。
func IsCancelled(err error) bool {
	return errors.Is(err, domain.ErrCancelled) || errors.Is(err, context.Canceled)
}
