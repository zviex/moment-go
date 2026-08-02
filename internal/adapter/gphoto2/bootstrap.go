package gphoto2

import (
	"fmt"
	"os"
	"path/filepath"
)

// libgphoto2 通过环境变量定位 camlib（相机驱动）与 iolib（端口驱动）目录：
//
//	CAMLIBS   -> 包含 ptp2.so 等相机驱动的目录
//	IOLIBS    -> 包含 ptpip.so、usb1.so 等端口驱动的目录
//
// 本仓库的 libgphoto2-android/lib 目录布局如下：
//
//	lib/
//	├── libgphoto2/2.5.34.1/          # camlib 目录
//	└── libgphoto2_port/0.12.2/       # iolib 目录
//
// Android 上这些 .so 会随 APK 打包，运行时需要将其解压到可读目录
// （例如应用 files 目录），再调用 Bootstrap 设置环境变量。
const (
	camlibVersionDir = "2.5.34.1"
	iolibVersionDir  = "0.12.2"
)

// Bootstrap 设置 libgphoto2 运行所需的环境变量。
//
//   - libRoot: 布局根目录（内含 libgphoto2/ 与 libgphoto2_port/ 子目录）；
//     为空时忽略。
//   - camlibDir: camlib 目录；为空时按 libRoot 推算。
//   - iolibDir: iolib 目录；为空时按 libRoot 推算。
//
// 环境变量是进程级的，本函数幂等；重复调用仅覆盖相同值。
func Bootstrap(libRoot, camlibDir, iolibDir string) error {
	if camlibDir == "" && libRoot != "" {
		camlibDir = filepath.Join(libRoot, "libgphoto2", camlibVersionDir)
	}
	if iolibDir == "" && libRoot != "" {
		iolibDir = filepath.Join(libRoot, "libgphoto2_port", iolibVersionDir)
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
