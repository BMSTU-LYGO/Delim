package maxapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"delim/pkg/metricsx"
)

var ErrNotConfigured = errors.New("MAX API client is not configured")

type Client struct {
	baseURL     string
	token       string
	httpClient  *http.Client
	limiter     *rateLimiter
	chatLimiter *perChatLimiter
	recorder    *metricsx.Recorder
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter time.Duration
}

type Bot struct {
	UserID           int64   `json:"user_id"`
	FirstName        string  `json:"first_name"`
	LastName         *string `json:"last_name"`
	Username         *string `json:"username"`
	IsBot            bool    `json:"is_bot"`
	LastActivityTime *int64  `json:"last_activity_time"`
	Description      *string `json:"description"`
	AvatarURL        string  `json:"avatar_url"`
	FullAvatarURL    string  `json:"full_avatar_url"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("MAX API returned status %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("MAX API returned status %d", e.StatusCode)
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		token:       token,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		limiter:     &rateLimiter{interval: time.Second / 30},
		chatLimiter: newPerChatLimiter(),
	}
}

// NewInstrumentedClient builds a Client wired to a metrics recorder. The
// recorder is consulted by the transport layer to surface MAX API errors
// with bounded labels (operation/status_class/result).
func NewInstrumentedClient(baseURL, token string, _ *slog.Logger) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		token:       token,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		limiter:     &rateLimiter{interval: time.Second / 30},
		chatLimiter: newPerChatLimiter(),
	}
}

// SetMetrics attaches a metrics recorder. Passing nil disables metrics. The
// method exists so callers can defer wiring the recorder until after the
// metrics listener has bound its port.
func (c *Client) SetMetrics(recorder *metricsx.Recorder) {
	c.recorder = recorder
}

func (c *Client) GetMe(ctx context.Context) (Bot, error) {
	var bot Bot
	if err := c.do(ctx, http.MethodGet, "/me", nil, nil, &bot, true); err != nil {
		return Bot{}, err
	}
	return bot, nil
}

type rateLimiter struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func (l *rateLimiter) wait(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if delay := time.Until(l.next); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	l.next = time.Now().Add(l.interval)
	return nil
}
