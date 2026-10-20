package gphoto2

// Option 配置 libgphoto2 后端。
type Option func(*options)

type options struct {
	libraryRoot string // CAMLIBS/IOLIBS 布局根目录
	camlibDir   string // 显式指定 camlib 目录（优先于 libraryRoot）
	iolibDir    string // 显式指定 iolib 目录（优先于 libraryRoot）
}

// WithLibraryRoot 设置 libgphoto2 运行库布局根目录。
//
// 不设置时使用平台默认行为：Linux / macOS 由 libgphoto2 搜索系统安装的驱动目录；
// Android 上驱动不在系统路径中，需要把随 APK 打包的 .so 解压到可读目录
// （例如应用 files 目录）。
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

func applyOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}
