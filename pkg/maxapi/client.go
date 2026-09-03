package maxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxResponseBody = 1 << 20

var ErrNotConfigured = errors.New("MAX API client is not configured")

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type APIError struct {
	StatusCode int
	Body       string
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
	return fmt.Sprintf("MAX API returned status %d: %s", e.StatusCode, e.Body)
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) GetMe(ctx context.Context) (Bot, error) {
	var bot Bot
	if err := c.do(ctx, http.MethodGet, "/me", nil, &bot); err != nil {
		return Bot{}, err
	}
	return bot, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, result any) error {
	if c.token == "" || c.baseURL == "" {
		return ErrNotConfigured
	}
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode MAX API request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return fmt.Errorf("create MAX API request: %w", err)
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("execute MAX API request: %w", err)
	}
	defer response.Body.Close()

	limitedBody := io.LimitReader(response.Body, maxResponseBody+1)
	data, err := io.ReadAll(limitedBody)
	if err != nil {
		return fmt.Errorf("read MAX API response: %w", err)
	}
	if len(data) > maxResponseBody {
		return errors.New("MAX API response body is too large")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return &APIError{StatusCode: response.StatusCode, Body: string(data)}
	}
	if result == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("decode MAX API response: %w", err)
	}
	return nil
}
