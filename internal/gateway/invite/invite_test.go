package invite

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

func TestInviteIsSignedBoundToGroupAndExpires(t *testing.T) {
	manager := NewManager("invite-secret")
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Second)
	token, issued, err := manager.Issue(42, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := manager.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if verified.GroupID != 42 || !verified.ExpiresAt.Equal(issued.ExpiresAt) {
		t.Fatalf("verified invite = %#v, issued = %#v", verified, issued)
	}

	raw, err := base64.RawURLEncoding.DecodeString(token[len(tokenPrefix):])
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint64(raw[1:9], 43)
	tampered := tokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	if _, err := manager.Verify(tampered); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("tampered group id error = %v, want invalid invite", err)
	}
}
