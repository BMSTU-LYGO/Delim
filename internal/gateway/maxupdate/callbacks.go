package maxupdate

import (
	"context"

	"delim/internal/gateway/callback"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxapi"
)

// confirmSettlementCallback handles a signed "confirm_settlement" callback
// button press. The signed token carries the settlement entity id; the MAX
// sender must be the settlement receiver, which Core enforces as the source of
// truth for permissions. Already-confirmed settlements are idempotent.
func (d *Dispatcher) confirmSettlementCallback(ctx context.Context, update Update) error {
	callbackID := update.EffectiveCallbackID()
	if callbackID == "" || d.callbacks == nil {
		return nil
	}
	token, err := d.callbacks.Verify(update.Callback.Payload)
	if err != nil {
		return d.maxAPI.AnswerCallback(ctx, update.EffectiveChatID(), callbackID, maxapi.AnswerCallbackRequest{Notification: "Не удалось проверить действие"})
	}
	if token.Action != callback.ConfirmSettlement {
		return nil
	}
	if update.Callback.User == nil {
		return nil
	}
	user, err := d.core.UpsertUser(ctx, &corev1.UpsertUserRequest{MaxUserId: update.Callback.User.UserID, FirstName: update.Callback.User.FirstName})
	if err != nil || user.GetUser().GetId() == 0 {
		return nil
	}
	// Core rejects a non-receiver actor and is idempotent on repeats.
	_, err = d.core.ConfirmSettlement(ctx, &corev1.ConfirmSettlementRequest{
		ActorUserId: user.GetUser().GetId(), SettlementId: token.EntityID,
	})
	notification := "Погашение подтверждено"
	if err != nil {
		notification = "Нельзя подтвердить это погашение"
		d.observeCallback("confirm_settlement", "rejected")
	} else {
		d.observeCallback("confirm_settlement", "ok")
	}
	return d.maxAPI.AnswerCallback(ctx, update.EffectiveChatID(), callbackID, maxapi.AnswerCallbackRequest{Notification: notification})
}
