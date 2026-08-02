package usecase

import (
	"testing"

	"ci.zevix.uk/moment-go/internal/domain"
)

func TestLifecycleTransitions(t *testing.T) {
	lc := NewLifecycle()
	if got := lc.State(); got != domain.StateDisconnected {
		t.Fatalf("initial state = %v, want disconnected", got)
	}

	if err := lc.transition(domain.StateConnected, domain.StateDisconnecting); err == nil {
		t.Fatal("transition from wrong state should fail")
	}

	if err := lc.transition(domain.StateDisconnected, domain.StateConnecting); err != nil {
		t.Fatalf("transition failed: %v", err)
	}
	if got := lc.State(); got != domain.StateConnecting {
		t.Fatalf("state = %v, want connecting", got)
	}

	if err := lc.transition(domain.StateConnecting, domain.StateConnected); err != nil {
		t.Fatalf("transition failed: %v", err)
	}
	if err := lc.transition(domain.StateConnected, domain.StateDisconnecting); err != nil {
		t.Fatalf("transition failed: %v", err)
	}
	if err := lc.transition(domain.StateDisconnecting, domain.StateDisconnected); err != nil {
		t.Fatalf("transition failed: %v", err)
	}
	if got := lc.State(); got != domain.StateDisconnected {
		t.Fatalf("final state = %v, want disconnected", got)
	}
}

func TestLifecycleFail(t *testing.T) {
	lc := NewLifecycle()
	_ = lc.transition(domain.StateDisconnected, domain.StateConnecting)
	lc.fail(domain.ErrCancelled)
	if got := lc.State(); got != domain.StateDisconnected {
		t.Fatalf("state = %v, want disconnected after failure", got)
	}
	if got := lc.Error(); got != domain.ErrCancelled {
		t.Fatalf("error = %v, want ErrCancelled", got)
	}
}
