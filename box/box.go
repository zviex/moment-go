package box

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"ci.zevix.uk/moment-go/adapter"
	"ci.zevix.uk/moment-go/adapter/gphoto2"
	"ci.zevix.uk/moment-go/config"
	"ci.zevix.uk/moment-go/pkg/moment"
	"ci.zevix.uk/moment-go/server"
	"ci.zevix.uk/moment-go/server/mnet"
	"ci.zevix.uk/moment-go/service/camera"
	"ci.zevix.uk/moment-go/service/connection"
	"ci.zevix.uk/moment-go/service/download"
	"ci.zevix.uk/moment-go/service/sync"
	"ci.zevix.uk/moment-go/storage"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
	downloadv1 "ci.zevix.uk/moment/moment-protogo/download/v1"
	syncv1 "ci.zevix.uk/moment/moment-protogo/sync/v1"
)

type Options struct {
	LibraryRoot string
	CamlibDir   string
	IolibDir    string

	// DataDir 是下载文件与历史数据库的根目录，默认 moment/data。
	DataDir string

	// SocketAddr 是 gRPC unix socket 地址；为空时使用平台默认地址。
	SocketAddr string
}

// Box 持有应用的全部长生命周期依赖。
type Box struct {
	Config    *config.Store
	Camera    *moment.Client
	Storage   *storage.Disk
	Downloads *download.Manager
	Sync      *sync.Recorder
	Server    *server.Server

	store   *download.Store
	network mnet.Net
	cancel  context.CancelFunc
}

func New(ctx context.Context, opts Options) (*Box, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)

	if opts.DataDir == "" {
		opts.DataDir = storage.DefaultDataDir()
	}

	backend, err := gphoto2.New(
		gphoto2.WithLibraryRoot(opts.LibraryRoot),
		gphoto2.WithLibraryDirs(opts.CamlibDir, opts.IolibDir),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("box: create gphoto2 backend: %w", err)
	}

	clients := moment.NewClient(backend.Connect, backend, backend.Close)
	resolver := adapter.NewTargetResolver(clients)

	disk := storage.NewDisk(opts.DataDir)

	store, err := download.OpenStore(ctx, filepath.Join(opts.DataDir, "download.db"))
	if err != nil {
		cancel()
		_ = clients.Close()
		return nil, fmt.Errorf("box: open download store: %w", err)
	}

	downloader := download.NewCameraDownloader(func() (adapter.Camera, error) {
		cam, err := clients.Camera()
		if err != nil {
			return nil, err
		}
		return cam, nil
	}, disk)
	manager := download.NewManager(ctx, store, downloader)
	recorder := sync.NewRecorder(ctx, nil, downloader)

	cfg := config.NewStore()
	network := networkFor(opts.SocketAddr)
	srv := server.NewServer(network)

	camerav1.RegisterCameraServiceServer(srv, camera.New(clients))
	connectionv1.RegisterCameraConnectionServiceServer(srv, connection.New(clients, resolver, cfg))
	downloadv1.RegisterDownloadServiceServer(srv, download.New(manager))
	syncv1.RegisterAutoSyncServiceServer(srv, sync.New(recorder))

	return &Box{
		Config:    cfg,
		Camera:    clients,
		Storage:   disk,
		Downloads: manager,
		Sync:      recorder,
		Server:    srv,
		store:     store,
		network:   network,
		cancel:    cancel,
	}, nil
}

func networkFor(addr string) mnet.Net {
	if addr == "" {
		return mnet.NewUnix()
	}
	return mnet.NewUnixAt(addr)
}

func (b *Box) Run(ctx context.Context) error {
	if b == nil || b.Server == nil {
		return errors.New("box: server is not initialized")
	}
	return b.Server.Run(ctx)
}

// Close 释放组合根持有的资源（相机会话、历史库、socket）。
func (b *Box) Close() error {
	if b == nil {
		return nil
	}

	if b.cancel != nil {
		b.cancel()
	}

	var errs []error
	if b.Camera != nil {
		if err := b.Camera.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if b.store != nil {
		if err := b.store.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if b.network != nil {
		if err := b.network.Cleanup(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
