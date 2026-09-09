package maxupdate

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"delim/internal/gateway/launch"
	postgresrepo "delim/internal/gateway/repository/postgres"
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
	GetBalance(context.Context, *corev1.GetBalanceRequest) (*corev1.GetBalanceResponse, error)
}

type Dispatcher struct {
	log        *slog.Logger
	store      *postgresrepo.Store
	maxAPI     *maxapi.Client
	core       coreClient
	launches   *launch.Manager
	miniAppURL string
	recorder   *metricsx.Recorder
	handlers   map[Type]handler
	botMu      sync.Mutex
	botID      int64
}

func NewDispatcher(store *postgresrepo.Store, maxAPI *maxapi.Client, core coreClient, launches *launch.Manager, log *slog.Logger, recorder *metricsx.Recorder, miniAppURL string) *Dispatcher {
	dispatcher := &Dispatcher{store: store, maxAPI: maxAPI, core: core, launches: launches, log: log, recorder: recorder, miniAppURL: strings.TrimRight(miniAppURL, "/")}
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
	if err := d.setChatStatus(ctx, update, "active"); err != nil {
		return err
	}
	return d.sendWelcome(ctx, update.EffectiveChatID())
}

func (d *Dispatcher) handleUserAdded(ctx context.Context, update Update) error {
	return d.logKnown(ctx, update)
}

func (d *Dispatcher) handleUserRemoved(ctx context.Context, update Update) error {
	return d.logKnown(ctx, update)
}

func (d *Dispatcher) handleMessageCreated(ctx context.Context, update Update) error {
	_ = d.logKnown(ctx, update)
	if update.Message == nil || update.Message.Sender != nil && update.Message.Sender.IsBot {
		return nil
	}
	switch strings.TrimSpace(update.Message.Body.Text) {
	case "/start":
		return d.sendWelcome(ctx, update.EffectiveChatID())
	case "/help":
		_, err := d.maxAPI.SendMessage(ctx, update.EffectiveChatID(), maxapi.NewMessage{Text: helpText})
		return err
	case "/new":
		return d.commandNewExpense(ctx, update)
	case "/balance":
		return d.commandBalance(ctx, update)
	default:
		return nil
	}
}

func (d *Dispatcher) handleMessageCallback(ctx context.Context, update Update) error {
	_ = d.logKnown(ctx, update)
	if update.Callback == nil {
		return nil
	}
	action, err := ParseCallbackPayload(update.Callback.Payload)
	if err != nil || action.Action != "help" || update.EffectiveCallbackID() == "" {
		return nil
	}
	return d.maxAPI.AnswerCallback(ctx, update.EffectiveChatID(), update.EffectiveCallbackID(), maxapi.AnswerCallbackRequest{Notification: helpText})
}

const (
	welcomeText = "Делим помогает вести совместные расходы и удобно делить их между участниками."
	helpText    = "Откройте мини-приложение Делим, чтобы работать с совместными расходами."
)

func (d *Dispatcher) sendWelcome(ctx context.Context, chatID int64) error {
	botID, err := d.getBotID(ctx)
	if err != nil {
		return err
	}
	_, err = d.maxAPI.SendMessage(ctx, chatID, maxapi.NewMessage{
		Text: welcomeText,
		Attachments: []maxapi.InlineKeyboard{{
			Type: "inline_keyboard",
			Payload: maxapi.InlineKeyboardPayload{Buttons: [][]maxapi.Button{{{
				Type:      "open_app",
				Text:      "Открыть Делим",
				ContactID: &botID,
			}}}},
		}},
	})
	return err
}

func (d *Dispatcher) getBotID(ctx context.Context) (int64, error) {
	d.botMu.Lock()
	defer d.botMu.Unlock()
	if d.botID != 0 {
		return d.botID, nil
	}
	bot, err := d.maxAPI.GetMe(ctx)
	if err != nil {
		return 0, err
	}
	d.botID = bot.UserID
	return d.botID, nil
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
