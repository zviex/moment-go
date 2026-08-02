package gphoto2

import (
	"errors"
	"strconv"
	"strings"
)

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
