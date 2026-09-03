package maxupdate

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	postgresrepo "delim/internal/gateway/repository/postgres"
)

const (
	workerInterval  = time.Second
	workerBatchSize = 20
	workerLease     = 30 * time.Second
	maxAttempts     = 5
	maxErrorLength  = 2000
)

type Worker struct {
	store      *postgresrepo.Store
	dispatcher *Dispatcher
	log        *slog.Logger
}

func NewWorker(store *postgresrepo.Store, dispatcher *Dispatcher, log *slog.Logger) *Worker {
	return &Worker{store: store, dispatcher: dispatcher, log: log}
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
	updates, err := w.store.ClaimUpdates(ctx, workerBatchSize, time.Now().Add(workerLease))
	if err != nil {
		if ctx.Err() == nil {
			w.log.Error("claim MAX updates", "error", err)
		}
		return
	}
	for _, stored := range updates {
		if ctx.Err() != nil {
			return
		}
		var update Update
		err := json.Unmarshal(stored.Payload, &update)
		if err == nil {
			err = w.dispatcher.Dispatch(ctx, update)
		}
		if err != nil {
			w.fail(ctx, stored, err)
			continue
		}
		if err := w.store.CompleteUpdate(ctx, stored.EventKey); err != nil && ctx.Err() == nil {
			w.log.Error("complete MAX update", "event_key", stored.EventKey, "error", err)
		}
	}
}

func (w *Worker) fail(ctx context.Context, update postgresrepo.StoredUpdate, processErr error) {
	delay := time.Second << min(update.Attempts-1, 6)
	message := processErr.Error()
	if len(message) > maxErrorLength {
		message = message[:maxErrorLength]
	}
	terminal := update.Attempts >= maxAttempts
	if err := w.store.FailUpdate(ctx, update.EventKey, message, time.Now().Add(delay), terminal); err != nil && ctx.Err() == nil {
		w.log.Error("reschedule MAX update", "event_key", update.EventKey, "error", err)
	}
}
