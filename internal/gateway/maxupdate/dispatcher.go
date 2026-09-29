package maxupdate

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"delim/internal/gateway/callback"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxapi"
	"delim/pkg/metricsx"
)

type handler func(context.Context, Update) error

// coreClient exposes the read-only Core operations the bot needs. Core stays the
// source of truth for membership/permissions and financial math; the Gateway
// only formats results.
type coreClient interface {
	UpsertUser(context.Context, *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error)
	ConfirmSettlement(context.Context, *corev1.ConfirmSettlementRequest) (*corev1.ConfirmSettlementResponse, error)
}

// store is the Dispatcher's storage surface (implemented by postgresrepo.Store).
type store interface {
	UpsertPersonalSubscription(ctx context.Context, maxUserID, chatID int64) error
	UpsertChat(ctx context.Context, chatID int64, isChannel bool, status string, eventAt time.Time) error
}

type Dispatcher struct {
	log       *slog.Logger
	store     store
	maxAPI    *maxapi.Client
	core      coreClient
	callbacks *callback.Manager
	recorder  *metricsx.Recorder
	handlers  map[Type]handler
}

func NewDispatcher(store store, maxAPI *maxapi.Client, core coreClient, callbacks *callback.Manager, log *slog.Logger, recorder *metricsx.Recorder) *Dispatcher {
	dispatcher := &Dispatcher{store: store, maxAPI: maxAPI, core: core, callbacks: callbacks, log: log, recorder: recorder}
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
	if update.User == nil || update.User.UserID == 0 || update.EffectiveChatID() == 0 {
		return nil
	}
	if err := d.store.UpsertPersonalSubscription(ctx, update.User.UserID, update.EffectiveChatID()); err != nil {
		return err
	}
	return nil
}

func (d *Dispatcher) handleUserAdded(ctx context.Context, update Update) error {
	// Delim membership is invite-driven inside the Mini App. A MAX chat member
	// event must never add somebody to a Delim group.
	return d.logKnown(ctx, update)
}

func (d *Dispatcher) handleUserRemoved(ctx context.Context, update Update) error {
	return d.logKnown(ctx, update)
}

func (d *Dispatcher) handleMessageCreated(ctx context.Context, update Update) error {
	if update.Message == nil {
		d.logMessageCreated(update, "")
		return nil
	}
	command := messageCommand(update.Message.Body.Text)
	d.logMessageCreated(update, command)
	if update.Message.Sender == nil || update.Message.Sender.IsBot || command != "start" {
		return nil
	}
	userID := update.Message.Sender.UserID
	if userID == 0 {
		d.log.Warn("MAX /start ignored: sender user id is missing")
		return nil
	}
	d.log.Info("MAX /start received", "sender_user_id", userID)
	if err := d.sendWelcomeToUser(ctx, userID); err != nil {
		d.log.Error("MAX welcome send failed", "user_id", userID, "error", err)
		d.observeBotCommand("start", "error")
		return err
	}
	d.log.Info("MAX welcome sent", "user_id", userID)
	d.observeBotCommand("start", "ok")
	return nil
}

func (d *Dispatcher) handleMessageCallback(ctx context.Context, update Update) error {
	_ = d.logKnown(ctx, update)
	if update.Callback == nil {
		return nil
	}
	if d.callbacks != nil && callback.LooksLike(update.Callback.Payload) {
		return d.confirmSettlementCallback(ctx, update)
	}
	return nil
}

const welcomeText = "Привет! Это Делим — сервис для удобного разделения общих расходов по чекам.\n\nИспользуя наш сервис, вы соглашаетесь на передачу данных чеков стороннему сервису для их обработки."

func messageCommand(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	command, _, _ := strings.Cut(fields[0], "@")
	switch command {
	case "/start":
		return "start"
	case "/help":
		return "help"
	case "/new":
		return "new"
	case "/balance":
		return "balance"
	default:
		if strings.HasPrefix(command, "/") {
			return "unknown"
		}
		return ""
	}
}

func (d *Dispatcher) sendWelcomeToUser(ctx context.Context, userID int64) error {
	_, err := d.maxAPI.SendMessageToUser(ctx, userID, maxapi.NewMessage{Text: welcomeText})
	return err
}

func (d *Dispatcher) logMessageCreated(update Update, command string) {
	var senderUserID, recipientChatID, recipientUserID int64
	if update.Message != nil {
		if update.Message.Sender != nil {
			senderUserID = update.Message.Sender.UserID
		}
		recipientChatID = update.Message.Recipient.ChatID
		recipientUserID = update.Message.Recipient.UserID
	}
	d.log.Info("MAX update received",
		"update_type", update.UpdateType,
		"chat_id", update.ChatID,
		"sender_user_id", senderUserID,
		"recipient_chat_id", recipientChatID,
		"recipient_user_id", recipientUserID,
		"command", command,
	)
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

func (d *Dispatcher) observeBotCommand(command, result string) {
	if d.recorder != nil {
		d.recorder.ObserveBotCommand(command, result)
	}
}

func (d *Dispatcher) observeCallback(action, result string) {
	if d.recorder != nil {
		d.recorder.ObserveCallback(action, result)
	}
}
