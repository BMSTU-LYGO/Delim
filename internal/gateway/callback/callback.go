// Package callback implements signed MAX callback payloads.
//
// A callback token is what a bot-issued inline button carries as its payload.
// The bot never trusts an unsigned entity id: the button payload embeds a
// signed token (version, action, entity_id, expiry, nonce, HMAC) that the
// Gateway verifies before acting on the callback.
package callback

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	// Version is the current callback token version.
	Version = 1
	prefix  = "dc_"
)

// Action is a signed callback intent.
type Action string

const (
	// ConfirmSettlement confirms a pending settlement from the MAX chat.
	ConfirmSettlement Action = "confirm_settlement"
)

func validAction(action Action) bool {
	return action == ConfirmSettlement
}

var (
	ErrNotConfigured      = errors.New("callback signing is not configured")
	ErrInvalidToken       = errors.New("invalid callback token")
	ErrExpiredToken       = errors.New("expired callback token")
	ErrUnsupportedVersion = errors.New("unsupported callback token version")
)

// Token is a decoded callback payload.
type Token struct {
	Version   int    `json:"v"`
	Action    Action `json:"action"`
	EntityID  int64  `json:"entity_id"`
	ExpiresAt int64  `json:"expires_at"`
	Nonce     string `json:"nonce"`
	Signature string `json:"sig"`
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

func (m *Manager) Configured() bool { return len(m.secret) > 0 }

// Issue signs a callback token for an action + entity.
func (m *Manager) Issue(action Action, entityID int64) (string, Token, error) {
	if !m.Configured() {
		return "", Token{}, ErrNotConfigured
	}
	if !validAction(action) || entityID <= 0 {
		return "", Token{}, ErrInvalidToken
	}
	ttl := m.ttl
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", Token{}, err
	}
	token := Token{
		Version:   Version,
		Action:    action,
		EntityID:  entityID,
		ExpiresAt: time.Now().Add(ttl).Unix(),
		Nonce:     base64.RawURLEncoding.EncodeToString(nonce),
	}
	token.Signature = m.sign(token)
	payload, err := json.Marshal(token)
	if err != nil {
		return "", Token{}, err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(payload), token, nil
}

// Verify validates signature, version and expiry of a callback token.
func (m *Manager) Verify(raw string) (Token, error) {
	if !m.Configured() {
		return Token{}, ErrNotConfigured
	}
	if !strings.HasPrefix(raw, prefix) {
		return Token{}, ErrInvalidToken
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return Token{}, ErrInvalidToken
	}
	var token Token
	if err := json.Unmarshal(decoded, &token); err != nil {
		return Token{}, ErrInvalidToken
	}
	if token.Version != Version || !validAction(token.Action) || token.EntityID <= 0 {
		return Token{}, ErrUnsupportedVersion
	}
	if !hmac.Equal([]byte(token.Signature), []byte(m.sign(token))) {
		return Token{}, ErrInvalidToken
	}
	if token.ExpiresAt <= 0 || time.Now().Unix() >= token.ExpiresAt {
		return Token{}, ErrExpiredToken
	}
	return token, nil
}

// LooksLike reports whether a string carries the callback token prefix.
func LooksLike(raw string) bool {
	return strings.HasPrefix(raw, prefix) && len(raw) > len(prefix)
}

// sign recomputes the HMAC over the token fields (signature excluded).
func (m *Manager) sign(token Token) string {
	mac := hmac.New(sha256.New, m.secret)
	signature := token.Signature
	token.Signature = ""
	encoded, _ := json.Marshal(token)
	_, _ = mac.Write(encoded)
	token.Signature = signature
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
