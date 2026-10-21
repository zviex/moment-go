package adapter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"ci.zevix.uk/moment-go/pkg/moment"
	connectionv1 "ci.zevix.uk/moment/moment-protogo/connection/v1"
)

const (
	// defaultPTPIPPort 是 PTP/IP 默认端口。
	defaultPTPIPPort = 15740
	// ptpipCameraModel 是能力表中 PTP/IP 相机的标准模型名。
	ptpipCameraModel = "PTP/IP Camera"
)

type TargetResolver interface {
	Resolve(ctx context.Context, opt *connectionv1.ConnectOption) (moment.Target, error)
}

type resolver struct {
	discoverer moment.Discoverer
}

func NewTargetResolver(discoverer moment.Discoverer) TargetResolver {
	return &resolver{discoverer: discoverer}
}

func (r *resolver) Resolve(ctx context.Context, opt *connectionv1.ConnectOption) (moment.Target, error) {
	if err := ctx.Err(); err != nil {
		return moment.Target{}, err
	}

	mode := opt.GetMode()
	if mode == connectionv1.ConnectionMode_CONNECTION_MODE_UNSPECIFIED {
		switch {
		case opt.GetPtp() != nil:
			mode = connectionv1.ConnectionMode_CONNECTION_MODE_PTPIP
		case opt.GetUsb() != nil:
			mode = connectionv1.ConnectionMode_CONNECTION_MODE_USB
		}
	}

	switch mode {
	case connectionv1.ConnectionMode_CONNECTION_MODE_PTPIP:
		return resolvePTPIP(opt)
	case connectionv1.ConnectionMode_CONNECTION_MODE_USB:
		return r.resolveUSB(ctx, opt)
	default: // CONNECTION_MODE_AUTO / UNSPECIFIED：自动探测第一个相机
		return r.firstDetected(ctx)
	}
}

func resolvePTPIP(opt *connectionv1.ConnectOption) (moment.Target, error) {
	ptp := opt.GetPtp()
	if ptp == nil {
		return moment.Target{}, errors.New("ptpip connect requires ptp payload")
	}
	addr, existingPort, err := splitHostPort(ptp.GetPortPath())
	if err != nil {
		return moment.Target{}, err
	}
	port := existingPort
	if port <= 0 {
		port = defaultPTPIPPort
	}
	target := moment.Target{
		PortPath: fmt.Sprintf("ptpip:%s:%d", addr, port),
		Model:    ptpipCameraModel,
	}
	slog.Debug("ptp/ip target resolved", "host", addr, "port", port, "port_path", target.PortPath)
	return target, nil
}

func (r *resolver) resolveUSB(ctx context.Context, opt *connectionv1.ConnectOption) (moment.Target, error) {
	usb := opt.GetUsb()
	if usb == nil || (usb.GetUsbVendorId() == 0 && usb.GetUsbProductId() == 0) {
		// 未指定 USB 设备标识时，取第一个探测到的相机。
		return r.firstDetected(ctx)
	}

	cams, err := r.detect(ctx)
	if err != nil {
		return moment.Target{}, err
	}
	for _, c := range cams {
		if usb.GetUsbVendorId() != 0 && c.USBVendorID != usb.GetUsbVendorId() {
			continue
		}
		if usb.GetUsbProductId() != 0 && c.USBProductID != usb.GetUsbProductId() {
			continue
		}
		return moment.Target{PortPath: c.PortPath, Model: c.Model}, nil
	}
	return moment.Target{}, fmt.Errorf(
		"usb camera %04x:%04x not detected",
		usb.GetUsbVendorId(), usb.GetUsbProductId(),
	)
}

func (r *resolver) firstDetected(ctx context.Context) (moment.Target, error) {
	cams, err := r.detect(ctx)
	if err != nil {
		return moment.Target{}, err
	}
	if len(cams) == 0 {
		return moment.Target{}, errors.New("no camera detected")
	}
	return moment.Target{PortPath: cams[0].PortPath, Model: cams[0].Model}, nil
}

func (r *resolver) detect(ctx context.Context) ([]moment.DetectedCamera, error) {
	if r.discoverer == nil {
		return nil, errors.New("camera discovery is unavailable")
	}
	return r.discoverer.Detect(ctx)
}

// splitHostPort 解析 PTP/IP 主机地址，兼容：
//
//	192.168.1.100
//	192.168.1.100:15740
//	ptpip:192.168.1.100[:port]
//	[fe80::1] / [fe80::1]:15740
//
// 返回 (地址, 端口)。端口未知时为 0，由调用方填充默认值。
func splitHostPort(host string) (addr string, port int, err error) {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "ptpip:")
	if host == "" {
		return "", 0, errors.New("ptpip connect requires host (IP or hostname)")
	}
	if strings.HasPrefix(host, "[") {
		// IPv6 方括号形式："[addr]" 或 "[addr]:port"。
		if i := strings.IndexByte(host, ']'); i >= 0 {
			addr = host[1:i]
			if rest := host[i+1:]; strings.HasPrefix(rest, ":") {
				if p, perr := strconv.Atoi(rest[1:]); perr == nil {
					port = p
				}
			}
		}
		if addr == "" {
			return "", 0, errors.New("invalid bracketed IPv6 host")
		}
		return addr, port, nil
	}
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		if p, perr := strconv.Atoi(host[i+1:]); perr == nil {
			return host[:i], p, nil
		}
	}
	return host, 0, nil
}
