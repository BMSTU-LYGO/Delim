// Package exportcap issues narrowly scoped, short-lived export download capabilities.
package exportcap

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const domain = "delim.export-download.v1"

var (
	ErrInvalid = errors.New("invalid export download capability")
	ErrExpired = errors.New("expired export download capability")
)

type Manager struct {
	key []byte
	now func() time.Time
}

type payload struct {
	ExportID int64 `json:"e"`
	ActorID  int64 `json:"a"`
	Expires  int64 `json:"x"`
}

func NewManager(secret string) *Manager {
	if secret == "" {
		return &Manager{now: time.Now}
	}
	// Derive a distinct signing key so a capability can never be mistaken for a
	// session token, even when both originate from the configured session secret.
	derived := hmac.New(sha256.New, []byte(secret))
	_, _ = derived.Write([]byte(domain + ".key"))
	return &Manager{key: derived.Sum(nil), now: time.Now}
}

func (m *Manager) Issue(exportID, actorID int64, ttl time.Duration) (string, error) {
	if len(m.key) == 0 || exportID < 1 || actorID < 1 || ttl <= 0 {
		return "", ErrInvalid
	}
	encoded, err := json.Marshal(payload{ExportID: exportID, ActorID: actorID, Expires: m.now().Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(encoded)
	return body + "." + base64.RawURLEncoding.EncodeToString(m.sign(body)), nil
}

func (m *Manager) Verify(token string) (exportID, actorID int64, err error) {
	if len(m.key) == 0 {
		return 0, 0, ErrInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, ErrInvalid
	}
	signature, decodeErr := base64.RawURLEncoding.DecodeString(parts[1])
	if decodeErr != nil || !hmac.Equal(signature, m.sign(parts[0])) {
		return 0, 0, ErrInvalid
	}
	raw, decodeErr := base64.RawURLEncoding.DecodeString(parts[0])
	if decodeErr != nil {
		return 0, 0, ErrInvalid
	}
	var value payload
	if json.Unmarshal(raw, &value) != nil || value.ExportID < 1 || value.ActorID < 1 || value.Expires < 1 {
		return 0, 0, ErrInvalid
	}
	if !m.now().Before(time.Unix(value.Expires, 0)) {
		return 0, 0, ErrExpired
	}
	return value.ExportID, value.ActorID, nil
}

func (m *Manager) sign(body string) []byte {
	h := hmac.New(sha256.New, m.key)
	_, _ = h.Write([]byte(domain + ".payload." + body))
	return h.Sum(nil)
}
