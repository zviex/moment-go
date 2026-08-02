// Package logging 提供跨层使用的日志抽象，避免各层直接依赖具体日志实现。
package logging

import "log"

// Logger 是领域层与应用层依赖的最小日志接口。
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Nop 是空实现，用于不关心日志的场景。
type Nop struct{}

func (Nop) Debugf(string, ...any) {}
func (Nop) Infof(string, ...any)  {}
func (Nop) Warnf(string, ...any)  {}
func (Nop) Errorf(string, ...any) {}

// Std 是基于标准库 log 的实现。
type Std struct {
	// Verbose 为 true 时输出 Debug 级别日志。
	Verbose bool
}

func NewStd(verbose bool) *Std { return &Std{Verbose: verbose} }

func (s *Std) Debugf(format string, args ...any) {
	if s.Verbose {
		log.Printf("[debug] "+format, args...)
	}
}
func (s *Std) Infof(format string, args ...any)  { log.Printf("[info] "+format, args...) }
func (s *Std) Warnf(format string, args ...any)  { log.Printf("[warn] "+format, args...) }
func (s *Std) Errorf(format string, args ...any) { log.Printf("[error] "+format, args...) }
