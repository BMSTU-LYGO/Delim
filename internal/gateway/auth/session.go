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
	ErrNotConfigured      = errors.New("session auth is not configured")
	ErrInvalidSession     = errors.New("invalid session")
	ErrExpiredSession     = errors.New("expired session")
	ErrUnsupportedVersion = errors.New("unsupported session version")
)

// TokenVersion is embedded in every session claim set so future token format
// changes can be rejected explicitly instead of silently misparsed.
const TokenVersion = 1

type Session struct {
	UserID    int64
	MAXUserID int64
	IssuedAt  time.Time
	ExpiresAt time.Time
	Invite    *InviteContext
	// ChatID is the MAX chat context captured from server-verified initData.
	// Zero means the session was not established from within a chat. It is
	// never sourced from a client-supplied body.
	ChatID int64
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
	Version   int            `json:"v"`
	UserID    int64          `json:"user_id"`
	MAXUserID int64          `json:"max_user_id"`
	IssuedAt  int64          `json:"issued_at"`
	ExpiresAt int64          `json:"expires_at"`
	Invite    *InviteContext `json:"invite,omitempty"`
	ChatID    int64          `json:"chat_id,omitempty"`
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

func (m *Manager) Configured() bool {
	return len(m.secret) > 0
}

func (m *Manager) Issue(userID, maxUserID int64) (string, Session, error) {
	return m.IssueWithInvite(userID, maxUserID, nil)
}

func (m *Manager) IssueWithInvite(userID, maxUserID int64, invite *InviteContext) (string, Session, error) {
	return m.IssueWithContext(userID, maxUserID, invite, 0)
}

// IssueWithContext creates a session carrying the invite context and the
// server-verified MAX chat context (chatID = 0 when not in a chat).
func (m *Manager) IssueWithContext(userID, maxUserID int64, invite *InviteContext, chatID int64) (string, Session, error) {
	if len(m.secret) == 0 {
		return "", Session{}, ErrNotConfigured
	}
	if userID == 0 || maxUserID == 0 || m.ttl <= 0 {
		return "", Session{}, ErrInvalidSession
	}

	now := time.Now()
	session := Session{UserID: userID, MAXUserID: maxUserID, IssuedAt: now, ExpiresAt: now.Add(m.ttl), Invite: invite, ChatID: chatID}
	payload, err := json.Marshal(claims{
		Version:   TokenVersion,
		UserID:    session.UserID,
		MAXUserID: session.MAXUserID,
		IssuedAt:  session.IssuedAt.Unix(),
		ExpiresAt: session.ExpiresAt.Unix(),
		Invite:    invite,
		ChatID:    chatID,
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
	if err := json.Unmarshal(payload, &value); err != nil || value.UserID == 0 || value.MAXUserID == 0 || value.IssuedAt <= 0 || value.ExpiresAt <= value.IssuedAt {
		return Session{}, ErrInvalidSession
	}
	if value.Version != TokenVersion {
		return Session{}, ErrUnsupportedVersion
	}

	session := Session{
		UserID:    value.UserID,
		MAXUserID: value.MAXUserID,
		IssuedAt:  time.Unix(value.IssuedAt, 0),
		ExpiresAt: time.Unix(value.ExpiresAt, 0),
		Invite:    value.Invite,
		ChatID:    value.ChatID,
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
