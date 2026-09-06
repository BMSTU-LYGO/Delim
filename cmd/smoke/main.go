package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"delim/internal/gateway/auth"
	coreclient "delim/internal/gateway/client/core"
	gatewayconfig "delim/internal/gateway/config"
	corev1 "delim/pkg/gen/core/v1"
)

const (
	smokeUserAMAXID int64 = 8_900_000_000_000_001
	smokeUserBMAXID int64 = 8_900_000_000_000_002
	maxJSONResponse       = 4 << 20
)

type options struct {
	gatewayURL string
	coreAddr   string
	stories    string
}

type actor struct {
	id        int64
	maxUserID int64
	token     string
}

type scenario struct {
	api          *apiClient
	core         *coreclient.Client
	actorA       actor
	actorB       actor
	groupID      int64
	expenseID    int64
	settlementID int64
}

type apiClient struct {
	baseURL string
	client  *http.Client
}

type responseError struct {
	status int
	body   string
}

func (e *responseError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d: %s", e.status, e.body)
}

func main() {
	if err := run(context.Background(), parseOptions()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseOptions() options {
	var value options
	flag.StringVar(&value.gatewayURL, "gateway-url", "http://localhost:8080", "Gateway base URL")
	flag.StringVar(&value.coreAddr, "core-addr", "localhost:50051", "Core gRPC address used only to provision local smoke users")
	flag.StringVar(&value.stories, "stories", "expense", "Comma-separated smoke stories")
	flag.Parse()
	return value
}

func run(parent context.Context, options options) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	cfg, err := gatewayconfig.Load("configs/gateway.yaml")
	if err != nil {
		return fmt.Errorf("load gateway config: %w", err)
	}
	if cfg.App.Env != "local" {
		return errors.New("smoke test user provisioning is available only when app.env=local")
	}
	stories := storySet(options.stories)
	if !stories["expense"] {
		return errors.New("the expense story is required")
	}
	smoke, err := newScenario(ctx, cfg, options)
	if err != nil {
		return err
	}
	defer smoke.close()
	defer smoke.cleanup()
	if err := smoke.verifyExpenseBalance(ctx); err != nil {
		return fmt.Errorf("expense balance story: %w", err)
	}
	fmt.Println("expense balance story: ok")
	if stories["settlement"] {
		if err := smoke.verifySettlement(ctx); err != nil {
			return fmt.Errorf("settlement story: %w", err)
		}
		fmt.Println("settlement story: ok")
	}
	return nil
}

func storySet(value string) map[string]bool {
	result := make(map[string]bool)
	for _, story := range strings.Split(value, ",") {
		if story = strings.TrimSpace(story); story != "" {
			result[story] = true
		}
	}
	return result
}

func newScenario(ctx context.Context, cfg gatewayconfig.Config, options options) (*scenario, error) {
	core, err := coreclient.New(options.coreAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to Core: %w", err)
	}
	manager := auth.NewManager(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)
	actorA, err := provisionActor(ctx, core, manager, smokeUserAMAXID, "Smoke A")
	if err != nil {
		_ = core.Close()
		return nil, err
	}
	actorB, err := provisionActor(ctx, core, manager, smokeUserBMAXID, "Smoke B")
	if err != nil {
		_ = core.Close()
		return nil, err
	}
	return &scenario{
		api: &apiClient{
			baseURL: strings.TrimRight(options.gatewayURL, "/"),
			client:  &http.Client{Timeout: 20 * time.Second},
		},
		core: core, actorA: actorA, actorB: actorB,
	}, nil
}

func provisionActor(ctx context.Context, core *coreclient.Client, sessions *auth.Manager, maxUserID int64, name string) (actor, error) {
	response, err := core.UpsertUser(ctx, &corev1.UpsertUserRequest{MaxUserId: maxUserID, FirstName: name})
	if err != nil {
		return actor{}, fmt.Errorf("provision smoke user: %w", err)
	}
	userID := response.GetUser().GetId()
	if userID <= 0 {
		return actor{}, errors.New("Core returned an invalid smoke user")
	}
	token, _, err := sessions.Issue(userID, maxUserID)
	if err != nil {
		return actor{}, fmt.Errorf("issue smoke session: %w", err)
	}
	return actor{id: userID, maxUserID: maxUserID, token: token}, nil
}

func (s *scenario) verifyExpenseBalance(ctx context.Context) error {
	var group struct {
		ID int64 `json:"id"`
	}
	if err := s.api.json(ctx, http.MethodPost, "/api/v1/groups", s.actorA.token, map[string]any{
		"name": fmt.Sprintf("smoke-%d", time.Now().UnixNano()),
	}, http.StatusCreated, &group); err != nil {
		return fmt.Errorf("create group: %w", err)
	}
	if group.ID <= 0 {
		return errors.New("create group returned an invalid id")
	}
	s.groupID = group.ID
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/members", s.groupID), s.actorA.token, map[string]any{
		"user_ids": []int64{s.actorB.id},
	}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("add user B: %w", err)
	}

	var expense struct {
		ID int64 `json:"id"`
	}
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/expenses", s.groupID), s.actorA.token, map[string]any{
		"payer_user_id": s.actorA.id,
		"amount_minor":  10_000,
		"currency":      "RUB",
		"description":   "Smoke expense",
		"expense_date":  time.Now().UTC().Format(time.RFC3339),
		"split_type":    "equal",
		"participants": []map[string]any{
			{"user_id": s.actorA.id, "value": 0},
			{"user_id": s.actorB.id, "value": 0},
		},
		"items": []any{},
	}, http.StatusCreated, &expense); err != nil {
		return fmt.Errorf("create expense: %w", err)
	}
	if expense.ID <= 0 {
		return errors.New("create expense returned an invalid id")
	}
	s.expenseID = expense.ID
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/expenses/%d/confirm", s.expenseID), s.actorA.token, nil, http.StatusOK, nil); err != nil {
		return fmt.Errorf("confirm expense: %w", err)
	}

	var balances []struct {
		UserID         int64  `json:"user_id"`
		Currency       string `json:"currency"`
		NetAmountMinor int64  `json:"net_amount_minor"`
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d/balance", s.groupID), s.actorA.token, nil, http.StatusOK, &balances); err != nil {
		return fmt.Errorf("get balance: %w", err)
	}
	want := map[int64]int64{s.actorA.id: 5_000, s.actorB.id: -5_000}
	for _, balance := range balances {
		if balance.Currency == "RUB" {
			if expected, ok := want[balance.UserID]; ok && expected == balance.NetAmountMinor {
				delete(want, balance.UserID)
			}
		}
	}
	if len(want) != 0 {
		return fmt.Errorf("unexpected RUB balances; unmatched values: %v", want)
	}

	var breakdown struct {
		Entries []struct {
			OperationID int64 `json:"operation_id"`
		} `json:"entries"`
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d/balance/%d", s.groupID, s.actorA.id), s.actorA.token, nil, http.StatusOK, &breakdown); err != nil {
		return fmt.Errorf("get balance breakdown: %w", err)
	}
	for _, entry := range breakdown.Entries {
		if entry.OperationID == s.expenseID {
			return nil
		}
	}
	return errors.New("balance breakdown does not contain the source expense")
}

func (s *scenario) verifySettlement(ctx context.Context) error {
	var settlement struct {
		ID int64 `json:"id"`
	}
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/settlements", s.groupID), s.actorB.token, map[string]any{
		"sender_user_id":   s.actorB.id,
		"receiver_user_id": s.actorA.id,
		"amount_minor":     5_000,
		"currency":         "RUB",
	}, http.StatusCreated, &settlement); err != nil {
		return fmt.Errorf("create settlement: %w", err)
	}
	if settlement.ID <= 0 {
		return errors.New("create settlement returned an invalid id")
	}
	s.settlementID = settlement.ID
	confirmPath := fmt.Sprintf("/api/v1/settlements/%d/confirm", s.settlementID)
	if err := s.api.json(ctx, http.MethodPost, confirmPath, s.actorB.token, nil, http.StatusForbidden, nil); err != nil {
		return fmt.Errorf("reject confirmation by sender: %w", err)
	}
	if err := s.api.json(ctx, http.MethodPost, confirmPath, s.actorA.token, nil, http.StatusOK, nil); err != nil {
		return fmt.Errorf("confirm settlement by receiver: %w", err)
	}

	var balances []struct {
		UserID         int64  `json:"user_id"`
		Currency       string `json:"currency"`
		NetAmountMinor int64  `json:"net_amount_minor"`
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d/balance", s.groupID), s.actorA.token, nil, http.StatusOK, &balances); err != nil {
		return fmt.Errorf("get settled balance: %w", err)
	}
	want := map[int64]int64{s.actorA.id: 0, s.actorB.id: 0}
	for _, balance := range balances {
		if balance.Currency == "RUB" {
			if expected, ok := want[balance.UserID]; ok && expected == balance.NetAmountMinor {
				delete(want, balance.UserID)
			}
		}
	}
	if len(want) != 0 {
		return fmt.Errorf("settlement did not clear RUB balances; unmatched values: %v", want)
	}

	var history struct {
		Settlements []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"settlements"`
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d/settlements", s.groupID), s.actorA.token, nil, http.StatusOK, &history); err != nil {
		return fmt.Errorf("list settlement history: %w", err)
	}
	for _, value := range history.Settlements {
		if value.ID == s.settlementID && value.Status == "confirmed" {
			return nil
		}
	}
	return errors.New("confirmed settlement is missing from history")
}

func (s *scenario) cleanup() {
	if s.groupID <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/archive", s.groupID), s.actorA.token, nil, http.StatusOK, nil)
}

func (s *scenario) close() {
	_ = s.core.Close()
}

func (c *apiClient) json(ctx context.Context, method, path, token string, input any, wantStatus int, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxJSONResponse+1))
	if err != nil {
		return err
	}
	if len(raw) > maxJSONResponse {
		return errors.New("Gateway JSON response is too large")
	}
	if response.StatusCode != wantStatus {
		return &responseError{status: response.StatusCode, body: strings.TrimSpace(string(raw))}
	}
	if output != nil && len(raw) != 0 {
		if err := json.Unmarshal(raw, output); err != nil {
			return fmt.Errorf("decode Gateway response: %w", err)
		}
	}
	return nil
}
