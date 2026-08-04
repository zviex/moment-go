package config

import configv1 "ci.zevix.uk/moment-go/protogo/config/v1"

var _defalut = &configv1.Config{}

func SetDefault(cfg *configv1.Config) {
	_defalut = cfg
}

func Default() *configv1.Config {
	return _defalut
}
