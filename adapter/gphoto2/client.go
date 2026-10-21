//go:build cgo

package gphoto2

/*
#include <gphoto2/gphoto2.h>
#include <gphoto2/gphoto2-camera.h>
#include <gphoto2/gphoto2-context.h>
#include <gphoto2/gphoto2-filesys.h>
#include <gphoto2/gphoto2-file.h>
#include <gphoto2/gphoto2-list.h>
#include <gphoto2/gphoto2-abilities-list.h>
#include <gphoto2/gphoto2-port-info-list.h>
#include <gphoto2/gphoto2-result.h>
#include <gphoto2/gphoto2-widget.h>
#include <stdlib.h>
#include <string.h>

// 复制 CameraFileInfo 中的 MIME 类型。
// 注意：info->file.type 的字段名 type 是 Go 关键字，cgo 无法直接访问，
// 因此通过 C 辅助函数读取；C 前导块按文件独立编译，辅助函数需与调用方同文件。
static void gp_file_info_mime(const CameraFileInfo *info, char *buf, size_t bufsize) {
	if (!info || !buf || bufsize <= 0) return;
	strncpy(buf, info->file.type, bufsize - 1);
	buf[bufsize - 1] = '\0';
}
*/
import "C"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"ci.zevix.uk/moment-go/adapter"
	"ci.zevix.uk/moment-go/pkg/moment"
	camerav1 "ci.zevix.uk/moment/moment-protogo/camera/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// 文件信息字段位掩码（gphoto2-filesys.h）。
const (
	gpFileInfoType        = 1 << 0
	gpFileInfoSize        = 1 << 2
	gpFileInfoWidth       = 1 << 3
	gpFileInfoHeight      = 1 << 4
	gpFileInfoPermissions = 1 << 5
	gpFileInfoStatus      = 1 << 6
	gpFileInfoMtime       = 1 << 7
)

// 文件权限位（gphoto2-filesys.h）。
const (
	gpFilePermRead   = 1 << 0
	gpFilePermDelete = 1 << 1
)

// GP2Client 是 libgphoto2 后端，提供标准化 Connect 与 Detect。
// 与平台无关：Linux / macOS 使用系统安装的 libgphoto2，Android 使用随包 .so。
type GP2Client struct {
	ctx *C.GPContext
}

// New 创建 libgphoto2 后端并初始化 GPContext。
func New(opts ...Option) (*GP2Client, error) {
	o := applyOptions(opts)
	if err := Bootstrap(o.libraryRoot, o.camlibDir, o.iolibDir); err != nil {
		return nil, err
	}
	ctx, err := newGPContext()
	if err != nil {
		return nil, err
	}
	return &GP2Client{ctx: ctx}, nil
}

// Close 释放 GPContext。
func (g *GP2Client) Close() error {
	freeGPContext(g.ctx)
	g.ctx = nil
	return nil
}

// Detect 加载端口与能力列表，探测可用相机（USB），并附上所有可用端口。
func (g *GP2Client) Detect(ctx context.Context) ([]adapter.DetectedCamera, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var pl *C.GPPortInfoList
	if err := resultToError(int(C.gp_port_info_list_new(&pl))); err != nil {
		return nil, err
	}
	defer C.gp_port_info_list_free(pl)
	if err := resultToError(int(C.gp_port_info_list_load(pl))); err != nil {
		return nil, err
	}

	var al *C.CameraAbilitiesList
	if err := resultToError(int(C.gp_abilities_list_new(&al))); err != nil {
		return nil, err
	}
	defer C.gp_abilities_list_free(al)
	if err := resultToError(int(C.gp_abilities_list_load(al, g.ctx))); err != nil {
		return nil, err
	}

	var list *C.CameraList
	if err := resultToError(int(C.gp_list_new(&list))); err != nil {
		return nil, err
	}
	defer C.gp_list_free(list)

	if err := resultToError(int(C.gp_abilities_list_detect(al, pl, list, g.ctx))); err != nil {
		return nil, err
	}

	n := int(C.gp_list_count(list))
	cams := make([]adapter.DetectedCamera, 0, n)
	for i := 0; i < n; i++ {
		var name, value *C.char
		if C.gp_list_get_name(list, C.int(i), &name) != 0 {
			continue
		}
		if C.gp_list_get_value(list, C.int(i), &value) != 0 {
			continue
		}
		cam := adapter.DetectedCamera{
			Model:    C.GoString(name),
			PortPath: C.GoString(value),
		}
		// 能力表中记录了每个模型的 USB VID/PID，用于按设备标识精确选择相机。
		if idx := int(C.gp_abilities_list_lookup_model(al, name)); idx >= 0 {
			var ab C.CameraAbilities
			if C.gp_abilities_list_get_abilities(al, C.int(idx), &ab) == 0 {
				cam.USBVendorID = uint32(ab.usb_vendor)
				cam.USBProductID = uint32(ab.usb_product)
			}
		}
		cams = append(cams, cam)
	}
	return cams, nil
}

// Connect 用标准化目标（端口路径 + 模型名）建立相机连接。
// 如何从用户请求推导 Target 由调用方的 TargetResolver 负责。
func (g *GP2Client) Connect(ctx context.Context, target *camerav1.Tartget) (adapter.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	portPath, model := target.PortPath, target.Model
	slog.Debug("connecting camera target", "port_path", portPath, "model", model)

	// 1. 加载端口信息列表并定位端口。
	// 端口信息由列表持有，需保证列表在 gp_camera_init 之前一直存活。
	portList, portInfo, err := g.lookupPort(portPath)
	if err != nil {
		return nil, err
	}
	defer C.gp_port_info_list_free(portList)

	// 2. 加载能力列表并定位相机模型。
	abilities, err := g.lookupAbilities(model)
	if err != nil {
		return nil, err
	}

	// 3. 创建相机对象并初始化（真正的握手发生在 gp_camera_init）。
	var cam *C.Camera
	if err := resultToError(int(C.gp_camera_new(&cam))); err != nil {
		return nil, err
	}
	if err := resultToError(int(C.gp_camera_set_abilities(cam, abilities))); err != nil {
		C.gp_camera_free(cam)
		return nil, err
	}
	if err := resultToError(int(C.gp_camera_set_port_info(cam, portInfo))); err != nil {
		C.gp_camera_free(cam)
		return nil, err
	}
	if err := resultToError(int(C.gp_camera_init(cam, g.ctx))); err != nil {
		C.gp_camera_free(cam)
		return nil, err
	}

	driver := ""
	if ab := (C.CameraAbilities{}); C.gp_camera_get_abilities(cam, &ab) == 0 {
		if m := C.GoString(&ab.model[0]); m != "" {
			model = m
		}
		driver = C.GoString(&ab.library[0])
	}

	slog.Debug("camera driver resolved", "driver", driver, "model", model, "port_path", portPath)

	return &GP2Camera{
		client:   g,
		cam:      cam,
		model:    model,
		driver:   driver,
		portPath: portPath,
	}, nil
}

// lookupPort 在端口列表中定位指定路径的端口。
// 返回的端口信息由列表持有：调用方必须保持列表存活（至少到 gp_camera_init 之后），
// 并在使用完毕后释放列表。
func (g *GP2Client) lookupPort(portPath string) (*C.GPPortInfoList, C.GPPortInfo, error) {
	var pl *C.GPPortInfoList
	if err := resultToError(int(C.gp_port_info_list_new(&pl))); err != nil {
		return nil, nil, err
	}
	if err := resultToError(int(C.gp_port_info_list_load(pl))); err != nil {
		C.gp_port_info_list_free(pl)
		return nil, nil, err
	}

	portC := C.CString(portPath)
	defer C.free(unsafe.Pointer(portC))

	idx := C.gp_port_info_list_lookup_path(pl, portC)
	if idx < 0 {
		C.gp_port_info_list_free(pl)
		return nil, nil, fmt.Errorf("port %q not found", portPath)
	}
	var info C.GPPortInfo
	if err := resultToError(int(C.gp_port_info_list_get_info(pl, idx, &info))); err != nil {
		C.gp_port_info_list_free(pl)
		return nil, nil, err
	}
	// 若命中正则匹配器（路径形如 "^ptpip:"），把实际路径写回端口信息，
	// 保证 ptp2 驱动能解析出主机地址。
	// 注意：gp_port_info_get_path 返回端口信息内部指针，不能 free；
	// gp_port_info_set_path 会复制传入的字符串。
	var path *C.char
	if C.gp_port_info_get_path(info, &path) == 0 && path != nil {
		if strings.HasPrefix(C.GoString(path), "^") {
			C.gp_port_info_set_path(info, portC)
		}
	}
	return pl, info, nil
}

// lookupAbilities 在能力列表中定位相机模型。
func (g *GP2Client) lookupAbilities(model string) (C.CameraAbilities, error) {
	var al *C.CameraAbilitiesList
	if err := resultToError(int(C.gp_abilities_list_new(&al))); err != nil {
		return C.CameraAbilities{}, err
	}
	defer C.gp_abilities_list_free(al)
	if err := resultToError(int(C.gp_abilities_list_load(al, g.ctx))); err != nil {
		return C.CameraAbilities{}, err
	}
	modelC := C.CString(model)
	defer C.free(unsafe.Pointer(modelC))
	idx := C.gp_abilities_list_lookup_model(al, modelC)
	if idx < 0 {
		return C.CameraAbilities{}, fmt.Errorf("%w: %q", moment.ErrModelNotFound, model)
	}
	var abilities C.CameraAbilities
	if err := resultToError(int(C.gp_abilities_list_get_abilities(al, idx, &abilities))); err != nil {
		return C.CameraAbilities{}, err
	}
	return abilities, nil
}

// GP2Camera 是已连接相机上的会话实现。
type GP2Camera struct {
	mu       sync.Mutex
	client   *GP2Client
	cam      *C.Camera
	model    string
	driver   string
	portPath string
	closed   bool
}

func (s *GP2Camera) Model() string    { return s.model }
func (s *GP2Camera) Driver() string   { return s.driver }
func (s *GP2Camera) PortPath() string { return s.portPath }

// requireOpen 校验会话可用并加锁。
func (s *GP2Camera) requireOpen() error {
	if s.closed || s.cam == nil {
		return moment.ErrNotConnected
	}
	return nil
}

// fileInfoLocked 调用 gp_camera_file_get_info 组装文件信息（需已持有锁）。
func (s *GP2Camera) fileInfoLocked(folderC, nameC *C.char) (*camerav1.FileEntry, error) {
	var ci C.CameraFileInfo
	if err := resultToError(int(C.gp_camera_file_get_info(s.cam, folderC, nameC, &ci, s.client.ctx))); err != nil {
		return nil, err
	}
	f := &camerav1.FileEntry{Name: C.GoString(nameC), Floder: C.GoString(folderC)}
	fileFields := int(ci.file.fields)
	if fileFields&gpFileInfoSize != 0 {
		f.Size = int64(ci.file.size)
	}
	if fileFields&gpFileInfoType != 0 {
		var mimeBuf [64]C.char
		C.gp_file_info_mime(&ci, &mimeBuf[0], 64)
		f.MimeType = C.GoString(&mimeBuf[0])
	}
	if fileFields&gpFileInfoWidth != 0 {
		f.Width = int32(ci.file.width)
	}
	if fileFields&gpFileInfoHeight != 0 {
		f.Height = int32(ci.file.height)
	}
	if fileFields&gpFileInfoStatus != 0 {
		f.Downloaded = int(ci.file.status) == 1 // GP_FILE_STATUS_DOWNLOADED
	}
	if fileFields&gpFileInfoPermissions != 0 {
		f.Permissions = int32(ci.file.permissions)
	}
	if fileFields&gpFileInfoMtime != 0 {
		if mt := int64(ci.file.mtime); mt > 0 {
			f.Mtime = timestamppb.New(time.Unix(int64(ci.file.mtime), 0))
		}
	}
	return f, nil
}

// Download 下载文件完整内容。
func (s *GP2Camera) Download(ctx context.Context, folder, name string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	folderC := C.CString(folder)
	defer C.free(unsafe.Pointer(folderC))
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	var cf *C.CameraFile
	if err := resultToError(int(C.gp_file_new(&cf))); err != nil {
		return nil, err
	}
	defer C.gp_file_free(cf)

	if err := resultToError(int(C.gp_camera_file_get(s.cam, folderC, nameC, C.GP_FILE_TYPE_NORMAL, cf, s.client.ctx))); err != nil {
		return nil, err
	}
	var data *C.char
	var size C.ulong
	if err := resultToError(int(C.gp_file_get_data_and_size(cf, &data, &size))); err != nil {
		return nil, err
	}
	if size == 0 {
		return []byte{}, nil
	}
	return C.GoBytes(unsafe.Pointer(data), C.int(size)), nil
}

// Open 打开文件内容。libgphoto2 当前一次性读取完整文件，因此以内存 reader 返回；
// 接口保留 io.ReadCloser 语义，便于未来替换为分块流式实现。
func (s *GP2Camera) Open(ctx context.Context, folder, name string) (io.ReadCloser, error) {
	data, err := s.Download(ctx, folder, name)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *GP2Camera) GetFileInfo(ctx context.Context, folder, name string) (*camerav1.FileEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	folderC := C.CString(folder)
	defer C.free(unsafe.Pointer(folderC))
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))
	return s.fileInfoLocked(folderC, nameC)
}

// DownloadTo 下载文件并写入 w。
func (s *GP2Camera) DownloadTo(ctx context.Context, folder, name string, w io.Writer) error {
	data, err := s.Download(ctx, folder, name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// GetInfo 聚合相机信息。
func (s *GP2Camera) GetInfo(ctx context.Context) (*camerav1.CameraSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireOpen(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	info := &camerav1.CameraSummary{
		Model:    s.model,
		Driver:   s.driver,
		PortPath: s.portPath,
	}

	// 摘要 / 说明 / 手册：驱动不支持时返回错误，忽略即可。
	var text C.CameraText
	if err := resultToError(int(C.gp_camera_get_summary(s.cam, &text, s.client.ctx))); err == nil {
		info.Summary = C.GoString(&text.text[0])
	}
	if err := resultToError(int(C.gp_camera_get_about(s.cam, &text, s.client.ctx))); err == nil {
		info.About = C.GoString(&text.text[0])
	}
	if err := resultToError(int(C.gp_camera_get_manual(s.cam, &text, s.client.ctx))); err == nil {
		info.Manual = C.GoString(&text.text[0])
	}

	// 存储信息。
	if storage, err := s.storageInfoLocked(); err == nil {
		info.Storage = storage
	} else if !errors.Is(err, moment.ErrUnsupported) {
		slog.Debug("storage info", "err", err)
	}

	// 配置树。
	if cfg, err := s.configLocked(); err == nil {
		info.Config = cfg.String()
	} else if !errors.Is(err, moment.ErrUnsupported) {
		slog.Debug("config tree", "err", err)
	}

	return info, nil
}

// storageInfoLocked 读取存储信息（需已持有锁）。
func (s *GP2Camera) storageInfoLocked() ([]*camerav1.StorageInfo, error) {
	var arr *C.CameraStorageInformation
	var n C.int
	if err := resultToError(int(C.gp_camera_get_storageinfo(s.cam, &arr, &n, s.client.ctx))); err != nil {
		return nil, err
	}
	if arr == nil || n <= 0 {
		return nil, nil
	}
	defer C.free(unsafe.Pointer(arr))

	// C 返回的是连续数组，转换为 Go 可索引的数组指针。
	elems := (*[1 << 30]C.CameraStorageInformation)(unsafe.Pointer(arr))[:n:n]
	out := make([]*camerav1.StorageInfo, 0, n)
	for _, e := range elems {
		st := &camerav1.StorageInfo{
			Basedir:     C.GoString(&e.basedir[0]),
			Label:       C.GoString(&e.label[0]),
			Description: C.GoString(&e.description[0]),
		}
		fields := int(e.fields)
		if fields&(1<<6) != 0 {
			st.CapacityKbytes = uint64(e.capacitykbytes)
		}
		if fields&(1<<7) != 0 {
			st.FreeKbytes = uint64(e.freekbytes)
		}
		if fields&(1<<8) != 0 {
			st.FreeImages = uint64(e.freeimages)
		}
		out = append(out, st)
	}
	return out, nil
}

// configLocked 读取并解析配置树（需已持有锁）。
func (s *GP2Camera) configLocked() (*moment.ConfigItem, error) {
	var root *C.CameraWidget
	if err := resultToError(int(C.gp_camera_get_config(s.cam, &root, s.client.ctx))); err != nil {
		return nil, err
	}
	defer C.gp_widget_free(root)
	return parseWidget(root)
}

// Close 断开相机并释放资源。
func (s *GP2Camera) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.cam == nil {
		return nil
	}
	if err := resultToError(int(C.gp_camera_exit(s.cam, s.client.ctx))); err != nil {
		C.gp_camera_free(s.cam)
		s.cam = nil
		return err
	}
	C.gp_camera_free(s.cam)
	s.cam = nil
	return nil
}

// parseWidget 递归解析 CameraWidget 树为领域配置树。
func parseWidget(w *C.CameraWidget) (*moment.ConfigItem, error) {
	item := &moment.ConfigItem{}
	var name, label, info *C.char
	C.gp_widget_get_name(w, &name)
	C.gp_widget_get_label(w, &label)
	C.gp_widget_get_info(w, &info)
	item.Name = C.GoString(name)
	item.Label = C.GoString(label)
	item.Info = C.GoString(info)

	var typ C.CameraWidgetType
	if C.gp_widget_get_type(w, &typ) != 0 {
		return nil, errors.New("failed to get widget type")
	}
	item.Type = widgetTypeToDomain(int(typ))

	var ro C.int
	C.gp_widget_get_readonly(w, &ro)
	item.ReadOnly = ro != 0

	// 按类型读取值。字符串类型写入 char*，数值类型写入对应标量。
	switch item.Type {
	case moment.ConfigText, moment.ConfigRadio, moment.ConfigMenu:
		var v *C.char
		if C.gp_widget_get_value(w, unsafe.Pointer(&v)) == 0 {
			item.Value = C.GoString(v)
		}
	case moment.ConfigToggle, moment.ConfigDate:
		var v C.int
		if C.gp_widget_get_value(w, unsafe.Pointer(&v)) == 0 {
			item.Value = int(v)
		}
	case moment.ConfigRange:
		var v C.float
		if C.gp_widget_get_value(w, unsafe.Pointer(&v)) == 0 {
			item.Value = float64(v)
		}
	}

	// 可选项（RADIO/MENU）。
	if n := int(C.gp_widget_count_choices(w)); n > 0 {
		for i := 0; i < n; i++ {
			var ch *C.char
			if C.gp_widget_get_choice(w, C.int(i), &ch) == 0 {
				item.Choices = append(item.Choices, C.GoString(ch))
			}
		}
	}

	// 子节点。
	if n := int(C.gp_widget_count_children(w)); n > 0 {
		for i := 0; i < n; i++ {
			var child *C.CameraWidget
			if C.gp_widget_get_child(w, C.int(i), &child) != 0 {
				continue
			}
			sub, err := parseWidget(child)
			if err == nil {
				item.Children = append(item.Children, sub)
			}
		}
	}
	return item, nil
}

// widgetTypeToDomain 把 C 的 CameraWidgetType 映射为领域类型。
func widgetTypeToDomain(t int) moment.ConfigType {
	switch t {
	case 0: // GP_WIDGET_WINDOW
		return moment.ConfigWindow
	case 1: // GP_WIDGET_SECTION
		return moment.ConfigSection
	case 2: // GP_WIDGET_TEXT
		return moment.ConfigText
	case 3: // GP_WIDGET_RANGE
		return moment.ConfigRange
	case 4: // GP_WIDGET_TOGGLE
		return moment.ConfigToggle
	case 5: // GP_WIDGET_RADIO
		return moment.ConfigRadio
	case 6: // GP_WIDGET_MENU
		return moment.ConfigMenu
	case 7: // GP_WIDGET_BUTTON
		return moment.ConfigButton
	case 8: // GP_WIDGET_DATE
		return moment.ConfigDate
	default:
		return moment.ConfigSection
	}
}

// joinFolder 拼接 gphoto2 风格的绝对目录路径。
func joinFolder(folder, name string) string {
	if folder == "" || folder == "/" {
		return "/" + name
	}
	return filepath.Join(folder, name)
}
