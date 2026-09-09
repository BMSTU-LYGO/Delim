package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const testSecret = "unit-test-session-secret-0000000000"

func newTestManager() *Manager {
	return NewManager(testSecret, time.Hour)
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	t.Parallel()
	manager := newTestManager()
	token, issued, err := manager.Issue(7, 70)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	session, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if session.UserID != issued.UserID || session.MAXUserID != issued.MAXUserID {
		t.Fatalf("session mismatch: %+v vs %+v", session, issued)
	}
}

func TestIssueWithContextPreservesVerifiedChat(t *testing.T) {
	t.Parallel()
	manager := newTestManager()
	token, issued, err := manager.IssueWithContext(7, 70, nil, 555123)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if issued.ChatID != 555123 {
		t.Fatalf("issued chat id = %d", issued.ChatID)
	}
	session, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if session.ChatID != 555123 {
		t.Fatalf("verified chat id = %d, want 555123", session.ChatID)
	}
	// Sessions issued without a chat must not carry one.
	plain, _, err := manager.Issue(7, 70)
	if err != nil {
		t.Fatalf("issue plain: %v", err)
	}
	if s, _ := manager.Verify(plain); s.ChatID != 0 {
		t.Fatalf("plain session leaked chat id %d", s.ChatID)
	}
}

func TestVerifyRejectsMalformedTokens(t *testing.T) {
	t.Parallel()
	manager := newTestManager()
	cases := []string{
		"",
		"no-dot",
		".",
		"a.b",
		base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".",
		base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".c2ln",
		"a.b.c",
	}
	for _, token := range cases {
		if _, err := manager.Verify(token); err == nil {
			t.Fatalf("expected rejection for malformed token %q", token)
		}
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	t.Parallel()
	manager := newTestManager()
	token, _, err := manager.Issue(7, 70)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	encodedPayload, signature, ok := strings.Cut(token, ".")
	if !ok {
		t.Fatal("token missing separator")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var value claims
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	value.UserID = 999
	tampered, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	forged := base64.RawURLEncoding.EncodeToString(tampered) + "." + signature
	if _, err := manager.Verify(forged); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected invalid session for tampered payload, got %v", err)
	}
}

func TestVerifyRejectsUnsupportedVersion(t *testing.T) {
	t.Parallel()
	manager := newTestManager()
	now := time.Now()
	payload, err := json.Marshal(claims{
		Version:   TokenVersion + 1,
		UserID:    7,
		MAXUserID: 70,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	token := encoded + "." + base64.RawURLEncoding.EncodeToString(manager.sign(encoded))
	if _, err := manager.Verify(token); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected unsupported version, got %v", err)
	}
}

func TestVerifyRejectsExpiredSession(t *testing.T) {
	t.Parallel()
	// Token expiry has unix-second resolution, so the shortest testable TTL
	// is one second plus a small sleep.
	manager := NewManager(testSecret, time.Second)
	token, _, err := manager.Issue(7, 70)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := manager.Verify(token); !errors.Is(err, ErrExpiredSession) {
		t.Fatalf("expected expired session, got %v", err)
	}
}

func TestVerifyWithoutSecretIsNotConfigured(t *testing.T) {
	t.Parallel()
	manager := NewManager("", time.Hour)
	if _, err := manager.Verify("anything"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected not configured, got %v", err)
	}
}
