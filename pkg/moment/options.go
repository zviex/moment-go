package moment

import "ci.zevix.uk/moment-go/internal/logging"

// Option 配置 Client 的可选项。
type Option func(*options)

type options struct {
	logger      logging.Logger
	libraryRoot string // CAMLIBS/IOLIBS 布局根目录
	camlibDir   string // 显式指定 camlib 目录（优先于 libraryRoot）
	iolibDir    string // 显式指定 iolib 目录（优先于 libraryRoot）
}

// WithLogger 设置日志实现；缺省为静默（logging.Nop）。
func WithLogger(l logging.Logger) Option {
	return func(o *options) {
		if l != nil {
			o.logger = l
		}
	}
}

// WithVerboseLog 使用标准库 log 并开启 Debug 输出。
func WithVerboseLog(verbose bool) Option {
	return func(o *options) {
		o.logger = logging.NewStd(verbose)
	}
}

// WithLibraryRoot 设置 libgphoto2 运行库布局根目录。
//
// Android 上需要把随 APK 打包的 .so 解压到可读目录（例如应用 files 目录），
// 目录内应包含：
//
//	<root>/libgphoto2/2.5.34.1/        # camlib（ptp2.so 等）
//	<root>/libgphoto2_port/0.12.2/     # iolib（ptpip.so、usb1.so 等）
//
// 该选项会把对应的 CAMLIBS / IOLIBS 环境变量设置好。
func WithLibraryRoot(root string) Option {
	return func(o *options) {
		o.libraryRoot = root
	}
}

// WithLibraryDirs 显式指定 camlib 与 iolib 目录（跳过布局推算）。
func WithLibraryDirs(camlibDir, iolibDir string) Option {
	return func(o *options) {
		o.camlibDir = camlibDir
		o.iolibDir = iolibDir
	}
}
