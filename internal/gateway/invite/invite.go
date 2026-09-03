package invite

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"
)

const (
	Version     byte = 1
	tokenPrefix      = "di_"
	nonceSize        = 12
	payloadSize      = 1 + 8 + 8 + nonceSize
	tokenSize        = payloadSize + sha256.Size
)

var (
	ErrNotConfigured      = errors.New("invite signing is not configured")
	ErrInvalidInvite      = errors.New("invalid invite")
	ErrExpiredInvite      = errors.New("expired invite")
	ErrUnsupportedVersion = errors.New("unsupported invite version")
)

type Invite struct {
	Version   byte
	GroupID   int64
	ExpiresAt time.Time
	Nonce     [nonceSize]byte
}

type Manager struct {
	secret []byte
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) Issue(groupID int64, expiresAt time.Time) (string, Invite, error) {
	if len(m.secret) == 0 {
		return "", Invite{}, ErrNotConfigured
	}
	if groupID <= 0 || !expiresAt.After(time.Now()) {
		return "", Invite{}, ErrInvalidInvite
	}
	value := Invite{Version: Version, GroupID: groupID, ExpiresAt: expiresAt}
	if _, err := rand.Read(value.Nonce[:]); err != nil {
		return "", Invite{}, err
	}
	payload := marshal(value)
	raw := append(payload, m.sign(payload)...)
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(raw), value, nil
}

func (m *Manager) Verify(token string) (Invite, error) {
	if len(m.secret) == 0 {
		return Invite{}, ErrNotConfigured
	}
	if !LooksLike(token) {
		return Invite{}, ErrInvalidInvite
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token[len(tokenPrefix):])
	if err != nil || len(raw) != tokenSize {
		return Invite{}, ErrInvalidInvite
	}
	payload := raw[:payloadSize]
	if !hmac.Equal(raw[payloadSize:], m.sign(payload)) {
		return Invite{}, ErrInvalidInvite
	}
	value := unmarshal(payload)
	if value.Version != Version {
		return Invite{}, ErrUnsupportedVersion
	}
	if value.GroupID <= 0 {
		return Invite{}, ErrInvalidInvite
	}
	if !time.Now().Before(value.ExpiresAt) {
		return Invite{}, ErrExpiredInvite
	}
	return value, nil
}

func LooksLike(token string) bool {
	return len(token) > len(tokenPrefix) && token[:len(tokenPrefix)] == tokenPrefix
}

func (m *Manager) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func marshal(value Invite) []byte {
	payload := make([]byte, payloadSize)
	payload[0] = value.Version
	binary.BigEndian.PutUint64(payload[1:9], uint64(value.GroupID))
	binary.BigEndian.PutUint64(payload[9:17], uint64(value.ExpiresAt.Unix()))
	copy(payload[17:], value.Nonce[:])
	return payload
}

func unmarshal(payload []byte) Invite {
	value := Invite{
		Version:   payload[0],
		GroupID:   int64(binary.BigEndian.Uint64(payload[1:9])),
		ExpiresAt: time.Unix(int64(binary.BigEndian.Uint64(payload[9:17])), 0),
	}
	copy(value.Nonce[:], payload[17:])
	return value
}
