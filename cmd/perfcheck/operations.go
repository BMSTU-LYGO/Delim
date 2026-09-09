package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type environment struct {
	baseURL    string
	tokenA     string
	tokenB     string
	userA      int64
	userB      int64
	groupID    int64
	expenseSeq int64
}

type operation struct {
	name    string
	target  time.Duration
	request func(context.Context, *environment) (*http.Request, error)
}

// operations covers the plan's core user stories: readiness, GET groups,
// GetBalance and a simple CreateExpense. Targets follow project requirements
// (ordinary API p95 <= 300ms, CreateExpense p95 <= 500ms).
func (env *environment) operations() []operation {
	const (
		normalTarget  = 300 * time.Millisecond
		expenseTarget = 500 * time.Millisecond
	)
	return []operation{
		{
			name:   "readiness",
			target: normalTarget,
			request: func(ctx context.Context, e *environment) (*http.Request, error) {
				return e.newRequest(ctx, http.MethodGet, "/health/ready", nil)
			},
		},
		{
			name:   "list-groups",
			target: normalTarget,
			request: func(ctx context.Context, e *environment) (*http.Request, error) {
				request, err := e.authorized(ctx, e.tokenA, http.MethodGet, "/api/v1/groups", nil)
				return request, err
			},
		},
		{
			name:   "get-balance",
			target: normalTarget,
			request: func(ctx context.Context, e *environment) (*http.Request, error) {
				return e.authorized(ctx, e.tokenA, http.MethodGet,
					fmt.Sprintf("/api/v1/groups/%d/balance", e.groupID), nil)
			},
		},
		{
			name:   "create-expense",
			target: expenseTarget,
			request: func(ctx context.Context, e *environment) (*http.Request, error) {
				return e.authorized(ctx, e.tokenA, http.MethodPost,
					fmt.Sprintf("/api/v1/groups/%d/expenses", e.groupID), e.expenseBody())
			},
		},
	}
}

func (env *environment) authorized(ctx context.Context, token, method, path string, body any) (*http.Request, error) {
	request, err := env.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	return request, nil
}

// expenseBody builds a unique, minimal equal-split expense so repeated
// CreateExpense timing runs operate on distinct resources.
func (env *environment) expenseBody() map[string]any {
	env.expenseSeq++
	return map[string]any{
		"payer_user_id": env.userA,
		"amount_minor":  1_000,
		"currency":      "RUB",
		"description":   fmt.Sprintf("perf expense %d", env.expenseSeq),
		"expense_date":  time.Now().UTC().Format(time.RFC3339),
		"split_type":    "equal",
		"participants": []map[string]any{
			{"user_id": env.userA, "value": 0},
			{"user_id": env.userB, "value": 0},
		},
		"items": []any{},
	}
}
