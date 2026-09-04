package domain

import "time"

type SettlementStatus string

const (
	SettlementPending   SettlementStatus = "pending"
	SettlementConfirmed SettlementStatus = "confirmed"
	SettlementCancelled SettlementStatus = "cancelled"
)

type Settlement struct {
	ID, GroupID, SenderUserID, ReceiverUserID, AmountMinor int64
	Currency                                               string
	Status                                                 SettlementStatus
	CreatedBy, Version                                     int64
	CreatedAt                                              time.Time
	ConfirmedAt                                            *time.Time
}

func ValidateSettlementConfirmation(actorID, receiverID int64, status SettlementStatus) error {
	if actorID != receiverID {
		return ErrForbidden
	}
	if status == SettlementPending || status == SettlementConfirmed {
		return nil
	}
	return ErrInvalidState
}
