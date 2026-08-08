package task_test

import (
	"testing"

	"ci.zevix.uk/moment-go/pkg/task"
)

func TestQuarterWatcherThresholds(t *testing.T) {
	var got []int32
	report := func(p task.Progress) { got = append(got, p.Percent()) }

	w := task.NewQuarterWatcher(task.Progress{URL: "u", Downloaded: 0, Total: 100}, report)
	w.Update(task.Progress{Downloaded: 25, Total: 100})
	w.Update(task.Progress{Downloaded: 26, Total: 100}) // 仍在 25%，不应重复触发
	w.Update(task.Progress{Downloaded: 50, Total: 100})
	w.Update(task.Progress{Downloaded: 75, Total: 100})
	w.Update(task.Progress{Downloaded: 100, Total: 100})

	want := []int32{0, 25, 50, 75, 100}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
