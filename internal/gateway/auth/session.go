package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotConfigured  = errors.New("session auth is not configured")
	ErrInvalidSession = errors.New("invalid session")
	ErrExpiredSession = errors.New("expired session")
)

type Session struct {
	MAXUserID int64
	IssuedAt  time.Time
	ExpiresAt time.Time
	Invite    *InviteContext
}

type InviteContext struct {
	GroupID   int64  `json:"group_id"`
	ExpiresAt int64  `json:"expires_at"`
	Nonce     string `json:"nonce"`
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

type claims struct {
	MAXUserID int64          `json:"max_user_id"`
	IssuedAt  int64          `json:"issued_at"`
	ExpiresAt int64          `json:"expires_at"`
	Invite    *InviteContext `json:"invite,omitempty"`
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

func (m *Manager) Configured() bool {
	return len(m.secret) > 0
}

func (m *Manager) Issue(maxUserID int64) (string, Session, error) {
	return m.IssueWithInvite(maxUserID, nil)
}

func (m *Manager) IssueWithInvite(maxUserID int64, invite *InviteContext) (string, Session, error) {
	if len(m.secret) == 0 {
		return "", Session{}, ErrNotConfigured
	}
	if maxUserID == 0 || m.ttl <= 0 {
		return "", Session{}, ErrInvalidSession
	}

	now := time.Now()
	session := Session{MAXUserID: maxUserID, IssuedAt: now, ExpiresAt: now.Add(m.ttl), Invite: invite}
	payload, err := json.Marshal(claims{
		MAXUserID: session.MAXUserID,
		IssuedAt:  session.IssuedAt.Unix(),
		ExpiresAt: session.ExpiresAt.Unix(),
		Invite:    invite,
	})
	if err != nil {
		return "", Session{}, err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.sign(encodedPayload)
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature), session, nil
}

func (m *Manager) Verify(token string) (Session, error) {
	if len(m.secret) == 0 {
		return Session{}, ErrNotConfigured
	}
	encodedPayload, encodedSignature, ok := strings.Cut(token, ".")
	if !ok || encodedPayload == "" || encodedSignature == "" || strings.Contains(encodedSignature, ".") {
		return Session{}, ErrInvalidSession
	}
	signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil || !hmac.Equal(signature, m.sign(encodedPayload)) {
		return Session{}, ErrInvalidSession
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	var value claims
	if err := json.Unmarshal(payload, &value); err != nil || value.MAXUserID == 0 || value.IssuedAt <= 0 || value.ExpiresAt <= value.IssuedAt {
		return Session{}, ErrInvalidSession
	}

	session := Session{
		MAXUserID: value.MAXUserID,
		IssuedAt:  time.Unix(value.IssuedAt, 0),
		ExpiresAt: time.Unix(value.ExpiresAt, 0),
		Invite:    value.Invite,
	}
	if !time.Now().Before(session.ExpiresAt) {
		return Session{}, ErrExpiredSession
	}
	if session.Invite != nil && time.Now().Unix() >= session.Invite.ExpiresAt {
		session.Invite = nil
	}
	return session, nil
}

func (m *Manager) sign(payload string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
