package maxupdate

import (
	"encoding/json"
	"errors"
)

type Type string

const (
	BotAdded        Type = "bot_added"
	BotRemoved      Type = "bot_removed"
	BotStarted      Type = "bot_started"
	UserAdded       Type = "user_added"
	UserRemoved     Type = "user_removed"
	MessageCreated  Type = "message_created"
	MessageCallback Type = "message_callback"
)

var ErrInvalidUpdate = errors.New("invalid MAX update")

type Update struct {
	UpdateType Type      `json:"update_type"`
	Timestamp  int64     `json:"timestamp"`
	ChatID     int64     `json:"chat_id,omitempty"`
	IsChannel  bool      `json:"is_channel,omitempty"`
	User       *User     `json:"user,omitempty"`
	Payload    string    `json:"payload,omitempty"`
	Message    *Message  `json:"message,omitempty"`
	Callback   *Callback `json:"callback,omitempty"`
	CallbackID string    `json:"callback_id,omitempty"`
}

type User struct {
	UserID    int64   `json:"user_id"`
	FirstName string  `json:"first_name,omitempty"`
	Name      string  `json:"name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	IsBot     bool    `json:"is_bot,omitempty"`
}

type Message struct {
	ID        string           `json:"mid,omitempty"`
	Timestamp int64            `json:"timestamp,omitempty"`
	Sender    *User            `json:"sender,omitempty"`
	Recipient MessageRecipient `json:"recipient,omitempty"`
	Body      MessageBody      `json:"body,omitempty"`
}

type MessageRecipient struct {
	ChatID int64 `json:"chat_id,omitempty"`
}

type MessageBody struct {
	Text string `json:"text,omitempty"`
}

type Callback struct {
	CallbackID string `json:"callback_id"`
	Payload    string `json:"payload,omitempty"`
	User       *User  `json:"user,omitempty"`
}

func Parse(data []byte) (Update, error) {
	var update Update
	if err := json.Unmarshal(data, &update); err != nil || update.UpdateType == "" {
		return Update{}, ErrInvalidUpdate
	}
	return update, nil
}

func (u Update) Known() bool {
	switch u.UpdateType {
	case BotAdded, BotRemoved, BotStarted, UserAdded, UserRemoved, MessageCreated, MessageCallback:
		return true
	default:
		return false
	}
}

func (u Update) EffectiveCallbackID() string {
	if u.Callback != nil && u.Callback.CallbackID != "" {
		return u.Callback.CallbackID
	}
	return u.CallbackID
}

func (u Update) EffectiveChatID() int64 {
	if u.ChatID != 0 {
		return u.ChatID
	}
	if u.Message != nil {
		return u.Message.Recipient.ChatID
	}
	return 0
}
