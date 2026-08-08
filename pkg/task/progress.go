package task

import "sync/atomic"

// quarterIndex 是 1/4 进度阈值（0%、25%、50%、75%、100%）。
var quarterIndex = []int32{0, 25, 50, 75, 100}

// QuarterWatcher 负责在进度跨越 1/4 阈值时触发回调，从而支持
// "1/4 计算事件推送"。它可容忍总字节数未知（Total<=0），此时退化为
// 只在起始(0)与完成(100)时触发。
type QuarterWatcher struct {
	last   atomic.Int32
	report func(Progress)
}

// NewQuarterWatcher 构造一个 1/4 进度观察器，并立即触发起始阈值。
// report 在跨越新阈值时被调用。
func NewQuarterWatcher(p Progress, report func(Progress)) *QuarterWatcher {
	w := &QuarterWatcher{report: report}
	w.last.Store(-1) // 尚未触发任何阈值
	w.check(p)
	return w
}

// Update 上报最新字节数，仅在跨越 1/4 阈值时触发回调。
func (w *QuarterWatcher) Update(p Progress) { w.check(p) }

func (w *QuarterWatcher) check(p Progress) {
	percent := p.Percent()

	idx := 0
	if percent >= 0 {
		for i, v := range quarterIndex {
			if percent >= v {
				idx = i
			}
		}
		// 未知总字节数时，只允许触发 0% 与 100%。
		if !p.Calculable() && idx > 0 && idx < len(quarterIndex)-1 {
			idx = 0
		}
	}

	if last := w.last.Load(); int32(idx) != last {
		w.last.Store(int32(idx))
		w.report(p)
	}
}

// Calculable 是否可计算百分比（总字节数已知且大于 0）。
func (p Progress) Calculable() bool { return p.Total > 0 }
