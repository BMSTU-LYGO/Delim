// Package launch implements signed Mini App *launch* tokens.
//
// A launch token encodes a navigation intent (open a group, the new-expense
// form, a balance, an expense or a settlement) that the bot embeds in an
// open_app link. It is explicitly NOT a membership grant: only invite tokens
// grant the right to join a group. A launch token is validated by the Gateway
// (HMAC + expiry) and, for group-scoped actions, the user's access to the
// target is verified against Core before the Mini App navigates.
package launch

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

// TokenVersion is embedded in every launch token.
const (
	TokenVersion = 1
	prefix       = "dl_"
)

// Action is the navigation intent carried by a launch token.
type Action string

const (
	ActionGroup      Action = "group"
	ActionNewExpense Action = "new_expense"
	ActionBalance    Action = "balance"
	ActionExpense    Action = "expense"
	ActionSettlement Action = "settlement"
)

func validAction(action Action) bool {
	switch action {
	case ActionGroup, ActionNewExpense, ActionBalance, ActionExpense, ActionSettlement:
		return true
	default:
		return false
	}
}

var (
	ErrNotConfigured      = errors.New("launch signing is not configured")
	ErrInvalidToken       = errors.New("invalid launch token")
	ErrExpiredToken       = errors.New("expired launch token")
	ErrUnsupportedVersion = errors.New("unsupported launch token version")
)

// Token is a decoded, verified launch intent.
type Token struct {
	Version   int    `json:"v"`
	GroupID   int64  `json:"group_id"`
	Action    Action `json:"action"`
	EntityID  int64  `json:"entity_id,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
	Nonce     string `json:"nonce"`
}

// RequiresGroup is true for actions that must resolve to a specific group whose
// membership the Gateway has to verify.
func (a Action) RequiresGroup() bool {
	switch a {
	case ActionGroup, ActionNewExpense, ActionBalance:
		return true
	default:
		return false
	}
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

func (m *Manager) Configured() bool { return len(m.secret) > 0 }

// Issue builds a signed launch token for the given intent.
func (m *Manager) Issue(action Action, groupID, entityID int64) (string, Token, error) {
	if !m.Configured() {
		return "", Token{}, ErrNotConfigured
	}
	if !validAction(action) {
		return "", Token{}, ErrInvalidToken
	}
	if groupID <= 0 && action.RequiresGroup() {
		return "", Token{}, ErrInvalidToken
	}
	ttl := m.ttl
	if ttl <= 0 {
		ttl = time.Hour
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", Token{}, err
	}
	token := Token{
		Version:   TokenVersion,
		GroupID:   groupID,
		Action:    action,
		EntityID:  entityID,
		ExpiresAt: time.Now().Add(ttl).Unix(),
		Nonce:     base64.RawURLEncoding.EncodeToString(nonce),
	}
	payload, err := json.Marshal(token)
	if err != nil {
		return "", Token{}, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.sign(encoded)
	return prefix + encoded + "." + base64.RawURLEncoding.EncodeToString(signature), token, nil
}

// Verify validates signature, version and expiry, and reports whether the token
// looks like a launch token (by prefix) so callers can distinguish it from an
// invite token.
func (m *Manager) Verify(raw string) (Token, error) {
	if !m.Configured() {
		return Token{}, ErrNotConfigured
	}
	encoded, encodedSignature, ok := strings.Cut(strings.TrimPrefix(raw, prefix), ".")
	if !ok || encoded == "" || encodedSignature == "" {
		return Token{}, ErrInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil || !hmac.Equal(signature, m.sign(encoded)) {
		return Token{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Token{}, ErrInvalidToken
	}
	var token Token
	if err := json.Unmarshal(payload, &token); err != nil {
		return Token{}, ErrInvalidToken
	}
	if token.Version != TokenVersion {
		return Token{}, ErrUnsupportedVersion
	}
	if !validAction(token.Action) || (token.Action.RequiresGroup() && token.GroupID <= 0) {
		return Token{}, ErrInvalidToken
	}
	if token.ExpiresAt <= 0 || time.Now().Unix() >= token.ExpiresAt {
		return Token{}, ErrExpiredToken
	}
	return token, nil
}

// LooksLike reports whether a start_param carries the launch token prefix.
func LooksLike(raw string) bool {
	return strings.HasPrefix(raw, prefix) && len(raw) > len(prefix)
}

func (m *Manager) sign(payload string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
