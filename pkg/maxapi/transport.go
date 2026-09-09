package maxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBody = 1 << 20
	requestTimeout  = 10 * time.Second
	maxSafeAttempts = 3
)

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, result any, safe bool) error {
	if c.token == "" || c.baseURL == "" {
		return ErrNotConfigured
	}
	requestContext, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var encodedBody []byte
	if body != nil {
		var err error
		encodedBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode MAX API request: %w", err)
		}
	}

	attempts := 1
	if safe {
		attempts = maxSafeAttempts
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if err := c.limiter.wait(requestContext); err != nil {
			return err
		}
		data, apiErr, err := c.execute(requestContext, method, path, query, encodedBody)
		if err != nil {
			if !safe || attempt == attempts-1 || requestContext.Err() != nil {
				return err
			}
			if err := wait(requestContext, time.Duration(1<<attempt)*200*time.Millisecond); err != nil {
				return err
			}
			continue
		}
		if apiErr == nil {
			if result == nil || len(data) == 0 {
				return nil
			}
			if err := json.Unmarshal(data, result); err != nil {
				return fmt.Errorf("decode MAX API response: %w", err)
			}
			return nil
		}
		if !safe || attempt == attempts-1 || !retryable(apiErr.StatusCode) {
			return apiErr
		}
		delay := apiErr.RetryAfter
		if delay <= 0 {
			delay = time.Duration(1<<attempt) * 200 * time.Millisecond
		}
		if err := wait(requestContext, delay); err != nil {
			return err
		}
	}
	return errors.New("MAX API request failed")
}

func (c *Client) execute(ctx context.Context, method, path string, query url.Values, body []byte) ([]byte, *APIError, error) {
	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		c.recordMAXAPIError(path, "transport", fmt.Errorf("parse MAX API URL: %w", err))
		return nil, nil, fmt.Errorf("parse MAX API URL: %w", err)
	}
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		c.recordMAXAPIError(path, "transport", fmt.Errorf("create MAX API request: %w", err))
		return nil, nil, fmt.Errorf("create MAX API request: %w", err)
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		c.recordMAXAPIError(path, "transport", fmt.Errorf("execute MAX API request: %w", err))
		return nil, nil, fmt.Errorf("execute MAX API request: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBody+1))
	closeErr := response.Body.Close()
	if readErr != nil {
		c.recordMAXAPIError(path, "transport", fmt.Errorf("read MAX API response: %w", readErr))
		return nil, nil, fmt.Errorf("read MAX API response: %w", readErr)
	}
	if closeErr != nil {
		c.recordMAXAPIError(path, "transport", fmt.Errorf("close MAX API response: %w", closeErr))
		return nil, nil, fmt.Errorf("close MAX API response: %w", closeErr)
	}
	if len(data) > maxResponseBody {
		c.recordMAXAPIError(path, "transport", errors.New("MAX API response body is too large"))
		return nil, nil, errors.New("MAX API response body is too large")
	}
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return data, nil, nil
	}
	apiErr := parseAPIError(response, data)
	c.recordMAXAPIError(path, httpStatusClass(response.StatusCode), apiErr)
	return nil, apiErr, nil
}

// recordMAXAPIError emits a bounded MAX API error metric. The path is mapped
// to a bounded operation label so the metric never carries raw URLs.
func (c *Client) recordMAXAPIError(path, statusClass string, err error) {
	if c.recorder == nil {
		return
	}
	c.recorder.ObserveMAXAPIError(maxAPIOperation(path), statusClass, err)
}

func maxAPIOperation(path string) string {
	switch {
	case strings.HasSuffix(path, "/me"):
		return "get_me"
	case strings.Contains(path, "/messages"):
		return "send_message"
	case strings.Contains(path, "/answers"):
		return "answer_callback"
	default:
		return "other"
	}
}

func httpStatusClass(status int) string {
	switch {
	case status < 200:
		return "1xx"
	case status < 300:
		return "2xx"
	case status < 400:
		return "3xx"
	case status < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

func parseAPIError(response *http.Response, data []byte) *APIError {
	var payload struct {
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
		Error   string          `json:"error"`
	}
	_ = json.Unmarshal(data, &payload)
	message := payload.Message
	if message == "" {
		message = payload.Error
	}
	if message == "" {
		message = strings.TrimSpace(string(data))
	}
	return &APIError{
		StatusCode: response.StatusCode,
		Code:       strings.Trim(string(payload.Code), `"`),
		Message:    message,
		RetryAfter: parseRetryAfter(response.Header.Get("Retry-After")),
	}
}

func parseRetryAfter(value string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return 0
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
