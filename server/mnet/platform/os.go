package platform

type Linux struct{}

func (Linux) OS() string {
	return "linux"
}

func (Linux) Addr() string {
	return "/tmp/moment_nkcon.sock"
}

type Android struct{}

func (Android) OS() string {
	return "android"
}

func (Android) Addr() string {
	return "@com.moment.nkcon"
}

type MacOS struct{}

func (MacOS) OS() string {
	return "macos"
}

func (MacOS) Addr() string {
	return "/tmp/moment_nkcon.sock"
}
