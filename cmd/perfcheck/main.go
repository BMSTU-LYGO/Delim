// Command perfcheck measures Gateway API latency for the core user stories
// and enforces the project's SLO targets on a stable local stack without OCR.
//
// It provisions two users directly through Core (as `cmd/smoke` does), issues
// local sessions with the shared gateway secret, then drives the Gateway HTTP
// API and reports median/p95 latency and error counts per operation.
//
// Targets (project requirements):
//   - ordinary API p95 <= 300ms
//   - simple CreateExpense p95 <= 500ms
//
// perfcheck exits non-zero when any operation records an error or breaches its
// p95 target. It requires the dev stack to be running (`make dev-up`). Total
// timed+warmed requests come from a single session, so keep -iterations well
// below the Gateway authenticated API rate limit.
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
	perfUserAMAXID int64 = 8_910_000_000_000_001
	perfUserBMAXID int64 = 8_910_000_000_000_002
	maxBodyBytes         = 1 << 20
)

type options struct {
	gatewayURL string
	coreAddr   string
	configPath string
	iterations int
	warmup     int
}

func parseOptions() options {
	var value options
	flag.StringVar(&value.gatewayURL, "gateway-url", "http://localhost:8080", "Gateway base URL")
	flag.StringVar(&value.coreAddr, "core-addr", "localhost:50051", "Core gRPC address used to provision perf users")
	flag.StringVar(&value.configPath, "config", "configs/gateway.yaml", "Gateway config used to issue local perf sessions")
	flag.IntVar(&value.iterations, "iterations", 30, "Timed requests per operation")
	flag.IntVar(&value.warmup, "warmup", 5, "Untimed warmup requests per operation")
	flag.Parse()
	return value
}

func main() {
	if err := run(context.Background(), parseOptions()); err != nil {
		fmt.Fprintln(os.Stderr, "perfcheck:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, options options) error {
	if options.iterations <= 0 {
		return errors.New("-iterations must be positive")
	}
	if options.warmup < 0 {
		return errors.New("-warmup must not be negative")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	cfg, err := gatewayconfig.Load(options.configPath)
	if err != nil {
		return fmt.Errorf("load gateway config: %w", err)
	}
	if cfg.App.Env != "local" {
		return errors.New("perfcheck session provisioning is available only when app.env=local")
	}

	core, err := coreclient.New(options.coreAddr, nil)
	if err != nil {
		return fmt.Errorf("connect to Core: %w", err)
	}
	defer core.Close()

	env := &environment{baseURL: strings.TrimRight(options.gatewayURL, "/")}
	if err := env.provision(ctx, core, cfg); err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	fmt.Printf("perfcheck: %d timed + %d warmup requests per operation (group %d)\n",
		options.iterations, options.warmup, env.groupID)
	fmt.Printf("%-16s %10s %10s %8s %10s\n", "operation", "median", "p95", "errors", "target")
	failures := 0
	for _, op := range env.operations() {
		samples, err := measure(ctx, client, op, env, options.iterations, options.warmup)
		if err != nil {
			return fmt.Errorf("%s warmup: %w", op.name, err)
		}
		summary := summarize(samples)
		breach := summary.p95 > op.target || summary.errors > 0
		if breach {
			failures++
		}
		status := "ok"
		if breach {
			status = "FAIL"
		}
		fmt.Printf("%-16s %10s %10s %8d %10s  %s\n",
			op.name, summary.median.Round(time.Millisecond), summary.p95.Round(time.Millisecond),
			summary.errors, op.target.Round(time.Millisecond), status)
	}
	if failures > 0 {
		return fmt.Errorf("%d operation(s) breached latency target or recorded errors", failures)
	}
	fmt.Println("perfcheck: all latency targets met")
	return nil
}

func (env *environment) provision(ctx context.Context, core *coreclient.Client, cfg gatewayconfig.Config) error {
	manager := auth.NewManager(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)
	userA, err := provisionActor(ctx, core, manager, perfUserAMAXID, "Perf A")
	if err != nil {
		return err
	}
	env.userA, env.tokenA = userA.id, userA.token
	userB, err := provisionActor(ctx, core, manager, perfUserBMAXID, "Perf B")
	if err != nil {
		return err
	}
	env.userB, env.tokenB = userB.id, userB.token

	client := &http.Client{Timeout: 10 * time.Second}
	var group struct {
		ID int64 `json:"id"`
	}
	if err := env.requestJSON(ctx, client, env.tokenA, http.MethodPost, "/api/v1/groups",
		map[string]any{"name": fmt.Sprintf("Delim perf-%d", time.Now().UnixNano())}, http.StatusCreated, &group); err != nil {
		return fmt.Errorf("create perf group: %w", err)
	}
	if group.ID <= 0 {
		return errors.New("create perf group returned an invalid id")
	}
	env.groupID = group.ID
	if err := env.requestJSON(ctx, client, env.tokenA, http.MethodPost,
		fmt.Sprintf("/api/v1/groups/%d/members", env.groupID),
		map[string]any{"user_ids": []int64{env.userB}}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("add perf member: %w", err)
	}
	return nil
}

type actorValue struct {
	id    int64
	token string
}

func provisionActor(ctx context.Context, core *coreclient.Client, manager *auth.Manager, maxUserID int64, name string) (actorValue, error) {
	response, err := core.UpsertUser(ctx, &corev1.UpsertUserRequest{MaxUserId: maxUserID, FirstName: name})
	if err != nil {
		return actorValue{}, fmt.Errorf("provision perf user: %w", err)
	}
	userID := response.GetUser().GetId()
	if userID <= 0 {
		return actorValue{}, errors.New("Core returned an invalid perf user")
	}
	token, _, err := manager.Issue(userID, maxUserID)
	if err != nil {
		return actorValue{}, fmt.Errorf("issue perf session: %w", err)
	}
	return actorValue{id: userID, token: token}, nil
}

func (env *environment) requestJSON(ctx context.Context, client *http.Client, token, method, path string, body any, expect int, out any) error {
	request, err := env.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes))
	if err != nil {
		return err
	}
	if response.StatusCode != expect {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))
	}
	if out != nil && len(payload) > 0 {
		return json.Unmarshal(payload, out)
	}
	return nil
}

func (env *environment) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, env.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}
