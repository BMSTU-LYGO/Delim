package callback

import (
	"errors"
	"testing"
	"time"
)

const secret = "callback-unit-test-secret-000000"

func TestIssueVerifyRoundTrip(t *testing.T) {
	t.Parallel()
	m := NewManager(secret, time.Hour)
	raw, token, err := m.Issue(ConfirmSettlement, 1234)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !LooksLike(raw) {
		t.Fatalf("token %q missing prefix", raw)
	}
	got, err := m.Verify(raw)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Action != ConfirmSettlement || got.EntityID != 1234 || got.Version != Version {
		t.Fatalf("decoded %+v != issued %+v", got, token)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	t.Parallel()
	m := NewManager(secret, time.Hour)
	raw, _, _ := m.Issue(ConfirmSettlement, 9)
	tampered := raw[:len(raw)-4] + "AAAA"
	if _, err := m.Verify(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected tamper rejection, got %v", err)
	}
	if _, err := m.Verify("dc_not_base64"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid for garbage, got %v", err)
	}
	if LooksLike("di_launchish") {
		t.Fatal("callback prefix must not collide with other token prefixes")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	t.Parallel()
	m := NewManager(secret, time.Nanosecond)
	raw, _, err := m.Issue(ConfirmSettlement, 1)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := m.Verify(raw); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected expired, got %v", err)
	}
}
