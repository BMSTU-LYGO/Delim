package notifications

import (
	"context"
	"encoding/json"
	"strconv"

	"delim/internal/gateway/repository/postgres"
)

// Outbox is the repository surface Notifier needs.
type Outbox interface {
	GetChatByGroup(ctx context.Context, groupID int64) (postgres.ChatGroupBinding, error)
	EnqueueNotification(ctx context.Context, dedupeKey, kind string, chatID int64, payload []byte) (bool, error)
}

// Notifier enqueues durable chat notifications for a bound group. It never
// blocks or fails the caller: binding/enqueue errors are ignored so a Core
// operation is never broken by a notification problem.
type Notifier struct {
	outbox Outbox
}

func NewNotifier(outbox Outbox) *Notifier {
	return &Notifier{outbox: outbox}
}

// NotifyGroup enqueues a notification for the group's bound chat (if any).
// dedupeKey makes repeated enqueues idempotent.
func (n *Notifier) NotifyGroup(ctx context.Context, groupID int64, kind, dedupeKey string, payload Payload) error {
	binding, err := n.outbox.GetChatByGroup(ctx, groupID)
	if err != nil {
		return nil // group not bound (or lookup error): nothing to notify
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	_, err = n.outbox.EnqueueNotification(ctx, dedupeKey, kind, binding.ChatID, encoded)
	return nil // best-effort: never fail the business operation
}

// ExpenseConfirmKey builds a stable dedupe key for a confirmed expense.
func ExpenseConfirmKey(expenseID int64) string {
	return "expense-confirm:" + strconv.FormatInt(expenseID, 10)
}

// SettlementKey builds a stable dedupe key for a created settlement.
func SettlementKey(settlementID int64) string {
	return "settlement:" + strconv.FormatInt(settlementID, 10)
}

// AdjustmentKey builds a stable dedupe key for an adjustment.
func AdjustmentKey(adjustmentID int64) string {
	return "adjustment:" + strconv.FormatInt(adjustmentID, 10)
}