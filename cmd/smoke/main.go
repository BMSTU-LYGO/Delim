package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"delim/internal/gateway/auth"
	coreclient "delim/internal/gateway/client/core"
	gatewayconfig "delim/internal/gateway/config"
	corev1 "delim/pkg/gen/core/v1"
)

const (
	smokeUserAMAXID int64 = 8_900_000_000_000_001
	smokeUserBMAXID int64 = 8_900_000_000_000_002
	smokeUserCMAXID int64 = 8_900_000_000_000_003
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
	api            *apiClient
	core           *coreclient.Client
	actorA         actor
	actorB         actor
	actorC         actor
	groupID        int64
	expenseID      int64
	settlementID   int64
	uploadMaxBytes int64
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
	if stories["health"] {
		client := &apiClient{
			baseURL: strings.TrimRight(options.gatewayURL, "/"),
			client:  &http.Client{Timeout: 5 * time.Second},
		}
		if err := verifyHealth(ctx, client); err != nil {
			return fmt.Errorf("health story: %w", err)
		}
		fmt.Println("health story: ok")
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
	if stories["adjustment"] {
		if !stories["settlement"] {
			return errors.New("the adjustment story requires the settlement story")
		}
		if err := smoke.verifyAdjustment(ctx); err != nil {
			return fmt.Errorf("adjustment story: %w", err)
		}
		fmt.Println("adjustment story: ok")
	}
	if stories["receipt"] {
		if err := smoke.verifyReceipt(ctx); err != nil {
			return fmt.Errorf("receipt OCR story: %w", err)
		}
		fmt.Println("receipt OCR story: ok")
	}
	if stories["document-unavailable"] {
		if err := smoke.verifyDocumentUnavailable(ctx); err != nil {
			return fmt.Errorf("document unavailable story: %w", err)
		}
		fmt.Println("document unavailable story: ok")
	}
	if stories["export"] {
		if !stories["settlement"] || !stories["adjustment"] {
			return errors.New("the export story requires the settlement and adjustment stories")
		}
		if err := smoke.verifyExport(ctx); err != nil {
			return fmt.Errorf("export story: %w", err)
		}
		fmt.Println("export story: ok")
	}
	return nil
}

func verifyHealth(ctx context.Context, client *apiClient) error {
	var response struct {
		Status   string `json:"status"`
		Core     string `json:"core"`
		Document string `json:"document"`
		Postgres string `json:"postgres"`
	}
	if err := client.json(ctx, http.MethodGet, "/health/ready", "", nil, http.StatusOK, &response); err != nil {
		return err
	}
	if response.Status != "ok" || response.Core != "ok" || response.Document != "ok" || response.Postgres != "ok" {
		return fmt.Errorf("unexpected readiness response: %+v", response)
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
	core, err := coreclient.New(options.coreAddr, nil)
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
	actorC, err := provisionActor(ctx, core, manager, smokeUserCMAXID, "Smoke C")
	if err != nil {
		_ = core.Close()
		return nil, err
	}
	return &scenario{
		api: &apiClient{
			baseURL: strings.TrimRight(options.gatewayURL, "/"),
			client:  &http.Client{Timeout: 20 * time.Second},
		},
		core: core, actorA: actorA, actorB: actorB, actorC: actorC, uploadMaxBytes: cfg.Document.UploadMaxSizeBytes(),
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
		"name": fmt.Sprintf("Делим smoke-%d", time.Now().UnixNano()),
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
		"description":   "Обед smoke",
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

func (s *scenario) verifyAdjustment(ctx context.Context) error {
	path := fmt.Sprintf("/api/v1/expenses/%d/adjustments", s.expenseID)
	request := map[string]any{
		"type":         "refund",
		"amount_minor": 1_000,
		"currency":     "RUB",
		"allocations": []map[string]any{
			{"user_id": s.actorB.id, "amount_minor": 1_000},
		},
	}
	if err := s.api.json(ctx, http.MethodPost, path, s.actorB.token, request, http.StatusForbidden, nil); err != nil {
		return fmt.Errorf("reject refund by unauthorized member: %w", err)
	}
	var adjustment struct {
		ID int64 `json:"id"`
	}
	if err := s.api.json(ctx, http.MethodPost, path, s.actorA.token, request, http.StatusCreated, &adjustment); err != nil {
		return fmt.Errorf("create refund: %w", err)
	}
	if adjustment.ID <= 0 {
		return errors.New("create refund returned an invalid id")
	}

	var balances []struct {
		UserID         int64  `json:"user_id"`
		Currency       string `json:"currency"`
		NetAmountMinor int64  `json:"net_amount_minor"`
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d/balance", s.groupID), s.actorA.token, nil, http.StatusOK, &balances); err != nil {
		return fmt.Errorf("get adjusted balance: %w", err)
	}
	want := map[int64]int64{s.actorA.id: -1_000, s.actorB.id: 1_000}
	for _, balance := range balances {
		if balance.Currency == "RUB" {
			if expected, ok := want[balance.UserID]; ok && expected == balance.NetAmountMinor {
				delete(want, balance.UserID)
			}
		}
	}
	if len(want) != 0 {
		return fmt.Errorf("refund did not update RUB balances; unmatched values: %v", want)
	}

	var expense struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/expenses/%d", s.expenseID), s.actorA.token, nil, http.StatusOK, &expense); err != nil {
		return fmt.Errorf("get original expense: %w", err)
	}
	if expense.ID != s.expenseID || expense.Status != "confirmed" {
		return errors.New("original confirmed expense was not preserved")
	}

	var history []struct {
		ID int64 `json:"id"`
	}
	if err := s.api.json(ctx, http.MethodGet, path, s.actorA.token, nil, http.StatusOK, &history); err != nil {
		return fmt.Errorf("list adjustment history: %w", err)
	}
	for _, value := range history {
		if value.ID == adjustment.ID {
			return nil
		}
	}
	return errors.New("refund is missing from adjustment history")
}

func (s *scenario) verifyReceipt(ctx context.Context) error {
	expensesBefore, err := s.expenseIDs(ctx)
	if err != nil {
		return fmt.Errorf("list expenses before OCR: %w", err)
	}
	imageBytes, err := smokePNG()
	if err != nil {
		return err
	}
	var created struct {
		Receipt struct {
			ID int64 `json:"id"`
		} `json:"receipt"`
		Job struct {
			ID int64 `json:"id"`
		} `json:"job"`
	}
	path := fmt.Sprintf("/api/v1/groups/%d/receipts", s.groupID)
	if err := s.api.upload(ctx, path, s.actorA.token, "smoke.png", "image/png", imageBytes, http.StatusCreated, &created); err != nil {
		return fmt.Errorf("upload receipt: %w", err)
	}
	if created.Receipt.ID <= 0 || created.Job.ID <= 0 {
		return errors.New("receipt upload returned invalid resource ids")
	}
	receiptID := created.Receipt.ID
	jobID := created.Job.ID
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/receipts/%d/retry", receiptID), s.actorA.token, nil, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("reject retry for active receipt: %w", err)
	}

	jobStatus, err := s.pollDocumentJob(ctx, jobID)
	if err != nil {
		return err
	}
	var result struct {
		Status string `json:"status"`
		Items  []any  `json:"items"`
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/receipts/%d/ocr", receiptID), s.actorA.token, nil, http.StatusOK, &result); err != nil {
		return fmt.Errorf("get OCR result: %w", err)
	}
	if result.Status != "ready" && result.Status != "failed" {
		return fmt.Errorf("unexpected terminal receipt status %q", result.Status)
	}
	expensesAfter, err := s.expenseIDs(ctx)
	if err != nil {
		return fmt.Errorf("list expenses after OCR: %w", err)
	}
	if !sameIDs(expensesBefore, expensesAfter) {
		return errors.New("OCR changed expenses without explicit user confirmation")
	}
	if jobStatus == "failed" {
		if result.Status != "failed" {
			return errors.New("failed OCR job did not mark receipt failed")
		}
		if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/receipts/%d/retry", receiptID), s.actorA.token, nil, http.StatusOK, nil); err != nil {
			return fmt.Errorf("retry failed receipt: %w", err)
		}
	} else if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/receipts/%d/retry", receiptID), s.actorA.token, nil, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("reject retry for ready receipt: %w", err)
	}
	if err := s.api.json(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/receipts/%d", receiptID), s.actorA.token, nil, http.StatusNoContent, nil); err != nil {
		return fmt.Errorf("delete receipt: %w", err)
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/receipts/%d", receiptID), s.actorA.token, nil, http.StatusNotFound, nil); err != nil {
		return fmt.Errorf("hide deleted receipt: %w", err)
	}

	if err := s.api.upload(ctx, path, s.actorA.token, "smoke.txt", "text/plain", []byte("not an image"), http.StatusBadRequest, nil); err != nil {
		return fmt.Errorf("reject unsupported upload: %w", err)
	}
	if s.uploadMaxBytes <= 0 {
		return errors.New("receipt upload size is not configured")
	}
	oversized := make([]byte, s.uploadMaxBytes+1)
	if err := s.api.upload(ctx, path, s.actorA.token, "oversized.png", "image/png", oversized, http.StatusRequestEntityTooLarge, nil); err != nil {
		return fmt.Errorf("reject oversized upload: %w", err)
	}
	return nil
}

func (s *scenario) expenseIDs(ctx context.Context) ([]int64, error) {
	var response struct {
		Expenses []struct {
			ID int64 `json:"id"`
		} `json:"expenses"`
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d/expenses", s.groupID), s.actorA.token, nil, http.StatusOK, &response); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(response.Expenses))
	for _, expense := range response.Expenses {
		ids = append(ids, expense.ID)
	}
	return ids, nil
}

func sameIDs(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[int64]int, len(left))
	for _, id := range left {
		counts[id]++
	}
	for _, id := range right {
		counts[id]--
		if counts[id] < 0 {
			return false
		}
	}
	return true
}

func (s *scenario) pollDocumentJob(ctx context.Context, jobID int64) (string, error) {
	path := fmt.Sprintf("/api/v1/document-jobs/%d", jobID)
	for {
		var job struct {
			Status string `json:"status"`
		}
		if err := s.api.json(ctx, http.MethodGet, path, s.actorA.token, nil, http.StatusOK, &job); err != nil {
			return "", fmt.Errorf("poll document job: %w", err)
		}
		switch job.Status {
		case "completed", "failed":
			return job.Status, nil
		case "pending", "processing":
		case "":
			return "", errors.New("document job response has no status")
		default:
			return "", fmt.Errorf("unexpected document job status %q", job.Status)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (s *scenario) verifyDocumentUnavailable(ctx context.Context) error {
	imageBytes, err := smokePNG()
	if err != nil {
		return err
	}
	return s.api.upload(ctx, fmt.Sprintf("/api/v1/groups/%d/receipts", s.groupID), s.actorA.token, "smoke.png", "image/png", imageBytes, http.StatusServiceUnavailable, nil)
}

func (s *scenario) verifyExport(ctx context.Context) error {
	formats := []struct {
		name        string
		contentType string
		signature   []byte
	}{
		{name: "csv", contentType: "text/csv; charset=utf-8"},
		{name: "pdf", contentType: "application/pdf", signature: []byte("%PDF")},
		{name: "xlsx", contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", signature: []byte("PK")},
	}
	var csvContent []byte
	var firstExportID int64
	for _, format := range formats {
		var created struct {
			ID int64 `json:"id"`
		}
		if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/exports", s.groupID), s.actorA.token, map[string]string{"format": format.name}, http.StatusCreated, &created); err != nil {
			return fmt.Errorf("create %s export: %w", format.name, err)
		}
		if created.ID <= 0 {
			return fmt.Errorf("create %s export returned an invalid id", format.name)
		}
		if firstExportID == 0 {
			firstExportID = created.ID
		}
		if err := s.pollExport(ctx, created.ID); err != nil {
			return fmt.Errorf("poll %s export: %w", format.name, err)
		}
		content, contentType, err := s.api.download(ctx, fmt.Sprintf("/api/v1/exports/%d/download", created.ID), s.actorA.token, http.StatusOK)
		if err != nil {
			return fmt.Errorf("download %s export: %w", format.name, err)
		}
		if contentType != format.contentType {
			return fmt.Errorf("%s export has content type %q", format.name, contentType)
		}
		if len(content) == 0 {
			return fmt.Errorf("%s export is empty", format.name)
		}
		if len(format.signature) != 0 && !bytes.HasPrefix(content, format.signature) {
			return fmt.Errorf("%s export has an invalid signature", format.name)
		}
		if format.name == "csv" {
			csvContent = content
		}
	}
	if !utf8.Valid(csvContent) {
		return errors.New("CSV export is not valid UTF-8")
	}
	for _, expected := range []string{
		"Делим",
		"Обед smoke",
		fmt.Sprintf("expense #%d", s.expenseID),
		fmt.Sprintf("settlement #%d", s.settlementID),
		"adjustment #",
	} {
		if !bytes.Contains(csvContent, []byte(expected)) {
			return fmt.Errorf("CSV export does not contain %q", expected)
		}
	}
	if err := s.api.json(ctx, http.MethodGet, fmt.Sprintf("/api/v1/exports/%d", firstExportID), s.actorC.token, nil, http.StatusNotFound, nil); err != nil {
		return fmt.Errorf("deny foreign export metadata: %w", err)
	}
	if _, _, err := s.api.download(ctx, fmt.Sprintf("/api/v1/exports/%d/download", firstExportID), s.actorC.token, http.StatusNotFound); err != nil {
		return fmt.Errorf("deny foreign export download: %w", err)
	}
	if err := s.api.json(ctx, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/exports", s.groupID), s.actorC.token, map[string]string{"format": "csv"}, http.StatusNotFound, nil); err != nil {
		return fmt.Errorf("deny foreign export creation: %w", err)
	}
	return nil
}

func (s *scenario) pollExport(ctx context.Context, exportID int64) error {
	path := fmt.Sprintf("/api/v1/exports/%d", exportID)
	for {
		var value struct {
			Status string `json:"status"`
		}
		if err := s.api.json(ctx, http.MethodGet, path, s.actorA.token, nil, http.StatusOK, &value); err != nil {
			return err
		}
		switch value.Status {
		case "ready":
			return nil
		case "failed":
			return errors.New("export failed")
		case "pending", "processing":
		default:
			return fmt.Errorf("unexpected export status %q", value.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func smokePNG() ([]byte, error) {
	value := image.NewRGBA(image.Rect(0, 0, 320, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 320; x++ {
			value.Set(x, y, color.White)
		}
	}
	for y := 30; y < 90; y++ {
		for x := 40; x < 280; x++ {
			if y < 36 || y > 83 || x < 46 || x > 273 {
				value.Set(x, y, color.Black)
			}
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, value); err != nil {
		return nil, fmt.Errorf("encode smoke image: %w", err)
	}
	return output.Bytes(), nil
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

func (c *apiClient) upload(ctx context.Context, path, token, filename, contentType string, content []byte, wantStatus int, output any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxJSONResponse+1))
	if err != nil {
		return err
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

func (c *apiClient) download(ctx context.Context, path, token string, wantStatus int) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := c.client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maxJSONResponse+1))
	if err != nil {
		return nil, "", err
	}
	if len(content) > maxJSONResponse {
		return nil, "", errors.New("Gateway download response is too large")
	}
	if response.StatusCode != wantStatus {
		return nil, "", &responseError{status: response.StatusCode, body: strings.TrimSpace(string(content))}
	}
	return content, response.Header.Get("Content-Type"), nil
}
