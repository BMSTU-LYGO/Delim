package launch

import (
	"errors"
	"testing"
	"time"
)

const secret = "launch-unit-test-secret-0000000000"

func TestIssueVerifyRoundTrip(t *testing.T) {
	t.Parallel()
	m := NewManager(secret, time.Hour)
	raw, issued, err := m.Issue(ActionNewExpense, 42, 0)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !LooksLike(raw) {
		t.Fatalf("token %q missing launch prefix", raw)
	}
	got, err := m.Verify(raw)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Action != ActionNewExpense || got.GroupID != 42 || got.Version != TokenVersion {
		t.Fatalf("decoded %+v != issued %+v", got, issued)
	}
}

func TestRequiresGroupValidation(t *testing.T) {
	t.Parallel()
	m := NewManager(secret, time.Hour)
	if _, _, err := m.Issue(ActionBalance, 0, 0); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for group action without group, got %v", err)
	}
	// expense action may carry no group binding requirement.
	if _, _, err := m.Issue(ActionExpense, 0, 7); err != nil {
		t.Fatalf("expense action without group should be allowed: %v", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	t.Parallel()
	m := NewManager(secret, time.Nanosecond)
	raw, _, err := m.Issue(ActionGroup, 5, 0)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := m.Verify(raw); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected expired, got %v", err)
	}
}

func TestVerifyRejectsTamperedAndForeign(t *testing.T) {
	t.Parallel()
	m := NewManager(secret, time.Hour)
	raw, _, _ := m.Issue(ActionGroup, 5, 0)
	if _, err := m.Verify(raw[:len(raw)-2] + "xx"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected tamper rejection, got %v", err)
	}
	if _, err := m.Verify("di_some_invite_like_token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid for invite-prefixed token, got %v", err)
	}
	// Launch and invite prefixes must be distinguishable.
	if LooksLike("di_whatever") {
		t.Fatal("launch LooksLike must not match invite prefix")
	}
}

func TestUnconfigured(t *testing.T) {
	t.Parallel()
	m := NewManager("", time.Hour)
	if m.Configured() {
		t.Fatal("empty secret must be unconfigured")
	}
	if _, _, err := m.Issue(ActionGroup, 1, 0); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected not configured, got %v", err)
	}
}
