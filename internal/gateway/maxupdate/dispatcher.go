package maxupdate

import (
	"context"
	"log/slog"
	"time"

	postgresrepo "delim/internal/gateway/repository/postgres"
)

type handler func(context.Context, Update) error

type Dispatcher struct {
	log      *slog.Logger
	store    *postgresrepo.Store
	handlers map[Type]handler
}

func NewDispatcher(store *postgresrepo.Store, log *slog.Logger) *Dispatcher {
	dispatcher := &Dispatcher{store: store, log: log}
	dispatcher.handlers = map[Type]handler{
		BotAdded:        dispatcher.handleBotAdded,
		BotRemoved:      dispatcher.handleBotRemoved,
		BotStarted:      dispatcher.handleBotStarted,
		UserAdded:       dispatcher.handleUserAdded,
		UserRemoved:     dispatcher.handleUserRemoved,
		MessageCreated:  dispatcher.handleMessageCreated,
		MessageCallback: dispatcher.handleMessageCallback,
	}
	return dispatcher
}

func (d *Dispatcher) Dispatch(ctx context.Context, update Update) error {
	handle, ok := d.handlers[update.UpdateType]
	if !ok {
		d.log.Debug("ignored unknown MAX update", "update_type", update.UpdateType)
		return nil
	}
	return handle(ctx, update)
}

func (d *Dispatcher) handleBotAdded(ctx context.Context, update Update) error {
	return d.setChatStatus(ctx, update, "active")
}

func (d *Dispatcher) handleBotRemoved(ctx context.Context, update Update) error {
	return d.setChatStatus(ctx, update, "removed")
}

func (d *Dispatcher) handleBotStarted(ctx context.Context, update Update) error {
	return d.setChatStatus(ctx, update, "active")
}

func (d *Dispatcher) handleUserAdded(ctx context.Context, update Update) error {
	return d.logKnown(ctx, update)
}

func (d *Dispatcher) handleUserRemoved(ctx context.Context, update Update) error {
	return d.logKnown(ctx, update)
}

func (d *Dispatcher) handleMessageCreated(ctx context.Context, update Update) error {
	return d.logKnown(ctx, update)
}

func (d *Dispatcher) handleMessageCallback(ctx context.Context, update Update) error {
	return d.logKnown(ctx, update)
}

func (d *Dispatcher) logKnown(_ context.Context, update Update) error {
	d.log.Info("MAX update received",
		"update_type", update.UpdateType,
		"timestamp", update.Timestamp,
		"chat_id", update.ChatID,
	)
	return nil
}

func (d *Dispatcher) setChatStatus(ctx context.Context, update Update, status string) error {
	eventAt := time.UnixMilli(update.Timestamp)
	if err := d.store.UpsertChat(ctx, update.ChatID, update.IsChannel, status, eventAt); err != nil {
		return err
	}
	return d.logKnown(ctx, update)
}
