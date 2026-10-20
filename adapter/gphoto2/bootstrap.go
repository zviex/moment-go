package gphoto2

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// libgphoto2 通过环境变量定位 camlib（相机驱动）与 iolib（端口驱动）目录：
//
//	CAMLIBS -> 包含 ptp2.so 等相机驱动的目录，形如 <root>/libgphoto2/<版本>/
//	IOLIBS  -> 包含 ptpip.so、usb1.so 等端口驱动的目录，形如 <root>/libgphoto2_port/<版本>/
//
// 默认（不传任何目录）不做任何设置：宿主平台由 libgphoto2 使用自身编译期默认路径
// 搜索系统安装的驱动目录，无需额外配置。
//
// Android 上驱动不在系统路径中，需要把随 APK 打包的驱动解压到可读目录
// （例如应用 files 目录），再用 Bootstrap / gphoto2.WithLibraryRoot 指定。
const (
	camlibParentDir = "libgphoto2"
	iolibParentDir  = "libgphoto2_port"
)

// Bootstrap 设置 libgphoto2 运行所需的驱动目录环境变量。
//
//   - libRoot: 布局根目录（内含 libgphoto2/ 与 libgphoto2_port/ 子目录）；
//     为空时不做任何设置，交由 libgphoto2 使用系统默认路径。
//   - camlibDir: camlib 目录；为空时按 libRoot 推算。
//   - iolibDir: iolib 目录；为空时按 libRoot 推算。
//
// 目录中的版本号由 Bootstrap 自行探测（如 2.5.34.1 / 0.12.2），无需硬编码。
// 环境变量是进程级的，本函数幂等；重复调用仅覆盖相同值。
func Bootstrap(libRoot, camlibDir, iolibDir string) error {
	if libRoot != "" {
		if camlibDir == "" {
			dir, err := resolveDriverDir(filepath.Join(libRoot, camlibParentDir))
			if err != nil {
				return fmt.Errorf("resolve camlib dir: %w", err)
			}
			camlibDir = dir
		}
		if iolibDir == "" {
			dir, err := resolveDriverDir(filepath.Join(libRoot, iolibParentDir))
			if err != nil {
				return fmt.Errorf("resolve iolib dir: %w", err)
			}
			iolibDir = dir
		}
	}
	if camlibDir != "" {
		if err := os.Setenv("CAMLIBS", camlibDir); err != nil {
			return fmt.Errorf("set CAMLIBS=%s: %w", camlibDir, err)
		}
	}
	if iolibDir != "" {
		if err := os.Setenv("IOLIBS", iolibDir); err != nil {
			return fmt.Errorf("set IOLIBS=%s: %w", iolibDir, err)
		}
	}
	return nil
}

// resolveDriverDir 在 libgphoto2 / libgphoto2_port 这类父目录下寻找实际存放驱动的目录：
// 优先取带版本号的一级子目录（2.5.34.1、0.12.2 等）；若父目录自身就是驱动目录
// （直接包含 .so / .dylib），则返回父目录本身。
func resolveDriverDir(parent string) (string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", fmt.Errorf("%s: %w", parent, err)
	}
	var children []string
	for _, e := range entries {
		if e.IsDir() {
			children = append(children, filepath.Join(parent, e.Name()))
			continue
		}
		if isSharedLib(e.Name()) {
			return parent, nil
		}
	}
	if len(children) == 0 {
		return "", fmt.Errorf("%s: no versioned driver directory found", parent)
	}
	sort.Strings(children)
	return children[0], nil
}

// isSharedLib 判断文件名是否是动态库（.so / .so.1 / .dylib）。
func isSharedLib(name string) bool {
	return strings.HasSuffix(name, ".so") || strings.Contains(name, ".so.") ||
		strings.HasSuffix(name, ".dylib")
}
