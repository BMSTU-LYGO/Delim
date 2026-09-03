package maxapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	perChatInterval = 500 * time.Millisecond
	maxPerChatWait  = 2 * time.Second
)

var ErrChatRateLimited = errors.New("MAX chat rate limit queue is full")

type Button struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Payload   string `json:"payload,omitempty"`
	URL       string `json:"url,omitempty"`
	WebApp    string `json:"web_app,omitempty"`
	ContactID *int64 `json:"contact_id,omitempty"`
}

type InlineKeyboard struct {
	Type    string                `json:"type"`
	Payload InlineKeyboardPayload `json:"payload"`
}

type InlineKeyboardPayload struct {
	Buttons [][]Button `json:"buttons"`
}

type NewMessage struct {
	Text        string           `json:"text,omitempty"`
	Attachments []InlineKeyboard `json:"attachments,omitempty"`
	Notify      *bool            `json:"notify,omitempty"`
	Format      string           `json:"format,omitempty"`
}

type Message struct {
	ID        string `json:"mid"`
	Timestamp int64  `json:"timestamp"`
	Body      struct {
		Text string `json:"text"`
	} `json:"body"`
}

type AnswerCallbackRequest struct {
	Message      *NewMessage `json:"message,omitempty"`
	Notification string      `json:"notification,omitempty"`
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, message NewMessage) (Message, error) {
	if err := c.chatLimiter.wait(ctx, chatID); err != nil {
		return Message{}, err
	}
	query := url.Values{"chat_id": []string{strconv.FormatInt(chatID, 10)}}
	var response struct {
		Message Message `json:"message"`
	}
	if err := c.do(ctx, http.MethodPost, "/messages", query, message, &response, false); err != nil {
		return Message{}, err
	}
	return response.Message, nil
}

func (c *Client) AnswerCallback(ctx context.Context, chatID int64, callbackID string, answer AnswerCallbackRequest) error {
	if err := c.chatLimiter.wait(ctx, chatID); err != nil {
		return err
	}
	query := url.Values{"callback_id": []string{callbackID}}
	var result operationResult
	if err := c.do(ctx, http.MethodPost, "/answers", query, answer, &result, false); err != nil {
		return err
	}
	return operationError(result)
}

type perChatLimiter struct {
	mu   sync.Mutex
	next map[int64]time.Time
}

func newPerChatLimiter() *perChatLimiter {
	return &perChatLimiter{next: make(map[int64]time.Time)}
}

func (l *perChatLimiter) wait(ctx context.Context, chatID int64) error {
	now := time.Now()
	l.mu.Lock()
	scheduled := now
	if next := l.next[chatID]; next.After(scheduled) {
		scheduled = next
	}
	delay := scheduled.Sub(now)
	if delay > maxPerChatWait {
		l.mu.Unlock()
		return ErrChatRateLimited
	}
	l.next[chatID] = scheduled.Add(perChatInterval)
	l.mu.Unlock()

	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
