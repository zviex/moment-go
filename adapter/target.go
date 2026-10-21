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

// ConnectOptions 是应用层发起连接时的请求参数，描述“想怎么连”，
// 而不是后端实际使用的端口 / 模型。它由 TargetResolver 归一化为 moment.Target。
type ConnectOptions struct {
	// Type 连接方式：CONNECTION_MODE_AUTO / USB / PTPIP，
	// 留空（UNSPECIFIED）等同自动探测。
	Type connectionv1.ConnectionMode
	// Model 相机模型名；PTP/IP 缺省为 "PTP/IP Camera"。
	Model string
	// Host PTP/IP 主机地址，可带端口或 "ptpip:" 前缀。
	Host string
	// Port PTP/IP 端口，0 表示使用 Host 中的端口或默认 15740。
	Port int
	// USBPort USB 端口路径（形如 "usb:001,002"）。
	USBPort string
}

// TargetResolver 负责把应用层的连接请求转换为后端可用的标准化连接目标。
// 不同连接方式（自动 / USB / PTP/IP）的参数构建逻辑都收敛在实现中。
type TargetResolver interface {
	Resolve(ctx context.Context, opts ConnectOptions) (moment.Target, error)
}

type resolver struct {
	discoverer moment.Discoverer
}

// NewTargetResolver 创建基于相机探测的默认解析器。
func NewTargetResolver(discoverer moment.Discoverer) TargetResolver {
	return &resolver{discoverer: discoverer}
}

// Resolve 把连接选项归一化为 (端口路径, 模型名)。
func (r *resolver) Resolve(ctx context.Context, opts ConnectOptions) (moment.Target, error) {
	if err := ctx.Err(); err != nil {
		return moment.Target{}, err
	}

	switch opts.Type {
	case connectionv1.ConnectionMode_CONNECTION_MODE_PTPIP:
		return resolvePTPIP(opts)
	case connectionv1.ConnectionMode_CONNECTION_MODE_USB:
		return r.resolveUSB(ctx, opts)
	default: // CONNECTION_MODE_AUTO / UNSPECIFIED：自动探测第一个相机
		return r.firstDetected(ctx)
	}
}

func resolvePTPIP(opts ConnectOptions) (moment.Target, error) {
	addr, existingPort, err := splitHostPort(strings.TrimSpace(opts.Host))
	if err != nil {
		return moment.Target{}, err
	}
	port := opts.Port
	if port <= 0 {
		port = existingPort
	}
	if port <= 0 {
		port = defaultPTPIPPort
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = ptpipCameraModel
	}
	target := moment.Target{
		PortPath: fmt.Sprintf("ptpip:%s:%d", addr, port),
		Model:    model,
	}
	slog.Debug("ptp/ip target resolved", "host", addr, "port", port, "port_path", target.PortPath)
	return target, nil
}

func (r *resolver) resolveUSB(ctx context.Context, opts ConnectOptions) (moment.Target, error) {
	portPath := strings.TrimSpace(opts.USBPort)
	model := strings.TrimSpace(opts.Model)
	if portPath == "" {
		// 未指定端口时，用模型（或第一个）匹配探测结果。
		cams, err := r.detect(ctx)
		if err != nil {
			return moment.Target{}, err
		}
		if len(cams) == 0 {
			return moment.Target{}, errors.New("no camera detected")
		}
		for _, c := range cams {
			if model == "" || c.Model == model {
				return moment.Target{PortPath: c.PortPath, Model: c.Model}, nil
			}
		}
		return moment.Target{}, fmt.Errorf("camera model %q not detected", model)
	}
	if model == "" {
		cams, err := r.detect(ctx)
		if err != nil {
			return moment.Target{}, err
		}
		for _, c := range cams {
			if c.PortPath == portPath {
				model = c.Model
				break
			}
		}
		if model == "" {
			return moment.Target{}, fmt.Errorf("no detected camera on port %q", portPath)
		}
	}
	return moment.Target{PortPath: portPath, Model: model}, nil
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
