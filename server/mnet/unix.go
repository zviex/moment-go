package mnet

type Net interface {
	Network() string
	Addr() string
}

var _default = &Unix{}

func DefaultNet() Net {
	return _default
}

type Unix struct{}

func (u Unix) Network() string {
	return "unix"
}

func (u Unix) Addr() string {
	return ""
}
