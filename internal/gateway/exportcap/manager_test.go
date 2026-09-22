package exportcap

import (
	"errors"
	"testing"
	"time"
)

func TestManagerIssuesExportScopedCapability(t *testing.T) {
	manager := NewManager("test-secret")
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	token, err := manager.Issue(42, 7, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	exportID, actorID, err := manager.Verify(token)
	if err != nil || exportID != 42 || actorID != 7 {
		t.Fatalf("Verify() = (%d, %d, %v), want (42, 7, nil)", exportID, actorID, err)
	}
	if _, _, err := NewManager("other-secret").Verify(token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("other manager Verify() error = %v, want invalid", err)
	}
}

func TestManagerRejectsExpiredAndUnconfiguredCapabilities(t *testing.T) {
	manager := NewManager("test-secret")
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	token, err := manager.Issue(42, 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return now.Add(time.Minute) }
	if _, _, err := manager.Verify(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify() error = %v, want expired", err)
	}
	if _, err := NewManager("").Issue(42, 7, time.Minute); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unconfigured Issue() error = %v, want invalid", err)
	}
	if _, _, err := NewManager("").Verify(token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unconfigured Verify() error = %v, want invalid", err)
	}
}
