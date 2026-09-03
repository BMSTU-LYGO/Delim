package maxapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

var ErrOperationFailed = errors.New("MAX API operation failed")

type Subscription struct {
	URL         string   `json:"url"`
	UpdateTypes []string `json:"update_types,omitempty"`
}

type CreateSubscriptionRequest struct {
	URL         string   `json:"url"`
	Secret      string   `json:"secret,omitempty"`
	UpdateTypes []string `json:"update_types,omitempty"`
}

type BotCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type operationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func (c *Client) GetSubscriptions(ctx context.Context) ([]Subscription, error) {
	var response struct {
		Subscriptions []Subscription `json:"subscriptions"`
	}
	if err := c.do(ctx, http.MethodGet, "/subscriptions", nil, nil, &response, true); err != nil {
		return nil, err
	}
	return response.Subscriptions, nil
}

func (c *Client) CreateSubscription(ctx context.Context, request CreateSubscriptionRequest) error {
	var result operationResult
	if err := c.do(ctx, http.MethodPost, "/subscriptions", nil, request, &result, true); err != nil {
		return err
	}
	return operationError(result)
}

func (c *Client) DeleteSubscription(ctx context.Context, webhookURL string) error {
	query := url.Values{"url": []string{webhookURL}}
	var result operationResult
	if err := c.do(ctx, http.MethodDelete, "/subscriptions", query, nil, &result, true); err != nil {
		return err
	}
	return operationError(result)
}

func (c *Client) SetBotCommands(ctx context.Context, commands []BotCommand) error {
	body := struct {
		Commands []BotCommand `json:"commands"`
	}{Commands: commands}
	return c.do(ctx, http.MethodPatch, "/me/commands", nil, body, nil, true)
}

func operationError(result operationResult) error {
	if result.Success {
		return nil
	}
	if result.Message != "" {
		return errors.New(result.Message)
	}
	return ErrOperationFailed
}
