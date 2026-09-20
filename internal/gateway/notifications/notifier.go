package notifications

import (
	"context"
	"encoding/json"
	"strconv"

	"delim/internal/gateway/callback"
	"delim/internal/gateway/launch"
	"delim/internal/gateway/repository/postgres"
)

// Outbox is the repository surface Notifier needs.
type Outbox interface {
	ListActivePersonalSubscriptions(ctx context.Context, maxUserIDs []int64) ([]postgres.PersonalSubscription, error)
	EnqueueNotification(ctx context.Context, dedupeKey, kind string, chatID int64, payload []byte) (bool, error)
}

// Notifier enqueues durable personal notifications. It never
// blocks or fails the caller: binding/enqueue errors are ignored so a Core
// operation is never broken by a notification problem.
type Notifier struct {
	outbox    Outbox
	callbacks *callback.Manager
}

func NewNotifier(outbox Outbox, callbacks *callback.Manager) *Notifier {
	return &Notifier{outbox: outbox, callbacks: callbacks}
}

// NotifyPersonal enqueues one notification per active personal subscription.
// The recipient-specific key preserves idempotency across retries and prevents
// any group-chat destination from entering the outbox.
func (n *Notifier) NotifyPersonal(ctx context.Context, maxUserIDs []int64, kind, dedupeKey string, payload Payload) error {
	subscriptions, err := n.outbox.ListActivePersonalSubscriptions(ctx, maxUserIDs)
	if err != nil {
		return nil
	}
	if kind == "settlement_created" && n.callbacks != nil {
		for _, spec := range payload.Buttons {
			if spec.Action == launch.ActionSettlement && spec.Entity > 0 {
				token, _, tokenErr := n.callbacks.Issue(callback.ConfirmSettlement, spec.Entity)
				if tokenErr == nil {
					payload.Buttons = append(payload.Buttons, ButtonSpec{
						Text: "Подтвердить", Kind: "callback", Entity: spec.Entity, Payload: token,
					})
				}
				break
			}
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	for _, subscription := range subscriptions {
		_, _ = n.outbox.EnqueueNotification(ctx, dedupeKey+":"+strconv.FormatInt(subscription.MAXUserID, 10), kind, subscription.ChatID, encoded)
	}
	return nil // best-effort: never fail the business operation
}

// ExpenseConfirmKey builds a stable dedupe key for a confirmed expense.
func ExpenseConfirmKey(expenseID int64) string {
	return "expense-confirm:" + strconv.FormatInt(expenseID, 10)
}

func ExpenseCreateKey(expenseID int64) string {
	return "expense-create:" + strconv.FormatInt(expenseID, 10)
}

func ExpenseUpdateKey(expenseID, version int64) string {
	return "expense-update:" + strconv.FormatInt(expenseID, 10) + ":" + strconv.FormatInt(version, 10)
}

// SettlementKey builds a stable dedupe key for a created settlement.
func SettlementKey(settlementID int64) string {
	return "settlement:" + strconv.FormatInt(settlementID, 10)
}

// AdjustmentKey builds a stable dedupe key for an adjustment.
func AdjustmentKey(adjustmentID int64) string {
	return "adjustment:" + strconv.FormatInt(adjustmentID, 10)
}
