package maxauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidInitData = errors.New("invalid MAX init data")
	ErrExpiredInitData = errors.New("expired MAX init data")
)

type InitData struct {
	UserID     int64
	FirstName  string
	LastName   string
	Username   string
	ChatID     *int64
	ChatType   string
	QueryID    string
	AuthDate   time.Time
	StartParam string
}

type InitDataVerifier struct {
	botToken string
	ttl      time.Duration
}

type initDataUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type initDataChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

func NewInitDataVerifier(botToken string, ttl time.Duration) *InitDataVerifier {
	return &InitDataVerifier{botToken: botToken, ttl: ttl}
}

func (v *InitDataVerifier) Configured() bool {
	return v.botToken != ""
}

func (v *InitDataVerifier) Verify(raw string) (InitData, error) {
	params, err := parseParams(raw)
	if err != nil {
		return InitData{}, err
	}

	providedHash, err := hex.DecodeString(params["hash"])
	if err != nil || len(providedHash) != sha256.Size {
		return InitData{}, ErrInvalidInitData
	}
	delete(params, "hash")

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+params[key])
	}

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMAC.Write([]byte(v.botToken))
	signatureMAC := hmac.New(sha256.New, secretMAC.Sum(nil))
	_, _ = signatureMAC.Write([]byte(strings.Join(lines, "\n")))
	if !hmac.Equal(signatureMAC.Sum(nil), providedHash) {
		return InitData{}, ErrInvalidInitData
	}

	return v.decode(params)
}

func (v *InitDataVerifier) decode(params map[string]string) (InitData, error) {
	seconds, err := strconv.ParseInt(params["auth_date"], 10, 64)
	if err != nil {
		return InitData{}, ErrInvalidInitData
	}
	authDate := time.Unix(seconds, 0)
	now := time.Now()
	if authDate.After(now) {
		return InitData{}, ErrInvalidInitData
	}
	if v.ttl > 0 && now.Sub(authDate) > v.ttl {
		return InitData{}, ErrExpiredInitData
	}

	var user initDataUser
	if err := json.Unmarshal([]byte(params["user"]), &user); err != nil || user.ID == 0 {
		return InitData{}, ErrInvalidInitData
	}
	if params["query_id"] == "" {
		return InitData{}, ErrInvalidInitData
	}

	data := InitData{
		UserID:     user.ID,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		Username:   user.Username,
		QueryID:    params["query_id"],
		AuthDate:   authDate,
		StartParam: params["start_param"],
	}
	if rawChat, ok := params["chat"]; ok {
		var chat initDataChat
		if err := json.Unmarshal([]byte(rawChat), &chat); err != nil || chat.ID == 0 {
			return InitData{}, ErrInvalidInitData
		}
		data.ChatID = &chat.ID
		data.ChatType = chat.Type
	}
	return data, nil
}

func parseParams(raw string) (map[string]string, error) {
	if raw == "" {
		return nil, ErrInvalidInitData
	}
	params := make(map[string]string)
	hashCount := 0
	for _, part := range strings.Split(raw, "&") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || key == "" {
			return nil, ErrInvalidInitData
		}
		if _, exists := params[key]; exists {
			return nil, ErrInvalidInitData
		}
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			return nil, fmt.Errorf("%w: decode %s", ErrInvalidInitData, key)
		}
		params[key] = decoded
		if key == "hash" {
			hashCount++
		}
	}
	if hashCount != 1 {
		return nil, ErrInvalidInitData
	}
	return params, nil
}
