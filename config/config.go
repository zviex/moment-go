package config

import (
	"sync"

	configv1 "ci.zevix.uk/moment/moment-protogo/config/v1"
)

type Store struct {
	mu  sync.RWMutex
	cfg *configv1.Config
}

func NewStore() *Store {
	return &Store{cfg: &configv1.Config{}}
}

func (s *Store) Set(cfg *configv1.Config) {
	if s == nil {
		return
	}
	if cfg == nil {
		cfg = &configv1.Config{}
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

// Get 返回当前配置（永不为 nil）。
func (s *Store) Get() *configv1.Config {
	if s == nil {
		return &configv1.Config{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}
