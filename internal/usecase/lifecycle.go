package usecase

import (
	"sync"

	"ci.zevix.uk/moment-go/internal/domain"
)

// Lifecycle 维护相机会话的生命周期状态机。
//
//	Disconnected --Connect--> Connecting --成功--> Connected
//	                                \--失败--> Disconnected
//	Connected --Disconnect--> Disconnecting --完成--> Disconnected
//
// 所有状态迁移在互斥锁保护下进行，保证并发安全。
type Lifecycle struct {
	mu    sync.RWMutex
	state domain.State
	err   error
}

func NewLifecycle() *Lifecycle {
	return &Lifecycle{state: domain.StateDisconnected}
}

// State 返回当前状态。
func (l *Lifecycle) State() domain.State {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state
}

// Error 返回最近一次失败的原因（若有）。
func (l *Lifecycle) Error() error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.err
}

// transition 仅在当前状态为 from 时切换到 to，否则返回 ErrInvalidState。
func (l *Lifecycle) transition(from, to domain.State) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != from {
		return domain.ErrInvalidState
	}
	l.state = to
	l.err = nil
	return nil
}

// fail 记录连接失败并回到未连接态。
func (l *Lifecycle) fail(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.err = err
	l.state = domain.StateDisconnected
}
