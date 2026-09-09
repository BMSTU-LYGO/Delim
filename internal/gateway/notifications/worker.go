// Package notifications delivers durable MAX chat notifications from the
// Gateway notification outbox. Delivery is asynchronous and must never break a
// Core operation: enqueue failures and sender errors only affect the
// notification itself.
package notifications

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"delim/internal/gateway/launch"
	postgresrepo "delim/internal/gateway/repository/postgres"
	"delim/pkg/maxapi"
	"delim/pkg/metricsx"
)

const (
	workerInterval  = time.Second
	workerBatchSize = 20
	workerLease     = 30 * time.Second
	maxAttempts     = 5
	maxErrorLength  = 2000
)

// ButtonSpec is a button attached to a notification message. It is either an
// open_app launch (default) or a signed callback carrying Payload.
type ButtonSpec struct {
	Text    string        `json:"text"`
	Kind    string        `json:"kind,omitempty"` // ""|"open"|"callback"
	Action  launch.Action `json:"action"`
	GroupID int64         `json:"group_id,omitempty"`
	Entity  int64         `json:"entity_id,omitempty"`
	Payload string        `json:"payload,omitempty"` // signed callback token
}

// Payload is the enqueued message body.
type Payload struct {
	Text    string       `json:"text"`
	Buttons []ButtonSpec `json:"buttons,omitempty"`
}

// Store is the outbox surface the worker needs.
type Store interface {
	ClaimNotifications(ctx context.Context, limit int, leaseUntil time.Time) ([]postgresrepo.StoredNotification, error)
	CompleteNotification(ctx context.Context, id int64) error
	FailNotification(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time, terminal bool) error
}

type Worker struct {
	store    Store
	maxAPI   *maxapi.Client
	launches *launch.Manager
	miniApp  string
	log      *slog.Logger
	recorder *metricsx.Recorder
}

func NewWorker(store Store, maxAPI *maxapi.Client, launches *launch.Manager, miniApp string, log *slog.Logger, recorder *metricsx.Recorder) *Worker {
	return &Worker{store: store, maxAPI: maxAPI, launches: launches, miniApp: miniApp, log: log, recorder: recorder}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()
	for {
		w.processBatch(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	// No MAX token configured: the Gateway keeps running; nothing is sent.
	if w.maxAPI == nil {
		return
	}
	items, err := w.store.ClaimNotifications(ctx, workerBatchSize, time.Now().Add(workerLease))
	if err != nil {
		if ctx.Err() == nil {
			w.log.Error("claim notifications", "error", err)
		}
		return
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		if err := w.send(ctx, item); err != nil {
			w.fail(ctx, item, err)
			continue
		}
		if err := w.store.CompleteNotification(ctx, item.ID); err != nil && ctx.Err() == nil {
			w.log.Error("complete notification", "id", item.ID, "error", err)
		}
	}
}

func (w *Worker) send(ctx context.Context, item postgresrepo.StoredNotification) error {
	var payload Payload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return err
	}
	message := maxapi.NewMessage{Text: payload.Text}
	if len(payload.Buttons) > 0 {
		keyboard := maxapi.InlineKeyboard{Type: "inline_keyboard"}
		for _, spec := range payload.Buttons {
			button := maxapi.Button{Type: "open_app", Text: spec.Text}
			if spec.Kind == "callback" && spec.Payload != "" {
				button = maxapi.Button{Type: "callback", Text: spec.Text, Payload: spec.Payload}
			} else {
				button.URL = w.buttonURL(spec)
			}
			keyboard.Payload.Buttons = append(keyboard.Payload.Buttons, []maxapi.Button{button})
		}
		message.Attachments = []maxapi.InlineKeyboard{keyboard}
	}
	_, err := w.maxAPI.SendMessage(ctx, item.ChatID, message)
	return err
}

func (w *Worker) buttonURL(spec ButtonSpec) string {
	if w.launches == nil || w.miniApp == "" {
		return ""
	}
	entity := spec.Entity
	token, _, err := w.launches.Issue(spec.Action, spec.GroupID, entity)
	if err != nil {
		return w.miniApp
	}
	if len(w.miniApp)+len(token)+len("?startapp=") > 2000 {
		return w.miniApp
	}
	return w.miniApp + "?startapp=" + token
}

func (w *Worker) fail(ctx context.Context, item postgresrepo.StoredNotification, processErr error) {
	delay := time.Second << min(item.Attempts-1, 6)
	message := processErr.Error()
	if len(message) > maxErrorLength {
		message = message[:maxErrorLength]
	}
	terminal := item.Attempts >= maxAttempts
	if err := w.store.FailNotification(ctx, item.ID, message, time.Now().Add(delay), terminal); err != nil && ctx.Err() == nil {
		w.log.Error("reschedule notification", "id", item.ID, "error", err)
	}
}

// min is a small local helper for the backoff shift.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
