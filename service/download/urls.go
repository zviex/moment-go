package download

import (
	"path"
	"strings"
)

// remote 前缀用于标识"相机内文件"的引用形式。
// 支持的两种形态都会被解析为相机的 folder + name：
//
//   - "moment:///store_00010001/DCIM/100CANON/IMG_0002.JPG"
//   - "/store_00010001/DCIM/100CANON/IMG_0002.JPG"
//
// folder 为去掉文件名后的目录路径，name 为文件名。
const urlScheme = "moment://"

// fileRef 是解析后的相机文件引用。
type fileRef struct {
	folder string // 相机内目录绝对路径，如 "/store_00010001/DCIM/100CANON"
	name   string // 文件名，如 "IMG_0002.JPG"
}

// parseFileRef 把一个远端传来的 url 解析为相机内的 folder + name。
func parseFileRef(url string) fileRef {
	p := url
	if i := strings.Index(p, urlScheme); i >= 0 {
		p = p[i+len(urlScheme):]
	}
	if p == "" {
		p = "/"
	}
	// 归一化：gphoto2 目录以 "/" 开头且不以 "/" 结尾。
	p = "/" + strings.TrimLeft(strings.TrimSpace(p), "/")
	p = path.Clean(p)
	dir, name := path.Split(p)
	dir = strings.TrimSuffix(dir, "/")
	if dir == "" {
		dir = "/"
	}
	return fileRef{folder: dir, name: name}
}

// localName 返回该引用写入 Disk 时使用的文件名。
func (ref fileRef) localName() string {
	if ref.name == "" {
		return "unnamed"
	}
	return ref.name
}

// displayName 用于事件与日志展示。
func (ref fileRef) displayName() string {
	return path.Join(ref.folder, ref.name)
}
