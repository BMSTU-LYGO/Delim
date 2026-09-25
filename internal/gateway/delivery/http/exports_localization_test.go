package http

import (
	"testing"

	corev1 "delim/pkg/gen/core/v1"
)

func TestExportStatusNamesAreRussian(t *testing.T) {
	expenseCases := map[corev1.ExpenseStatus]string{
		corev1.ExpenseStatus_EXPENSE_STATUS_PENDING:     "ожидает подтверждения",
		corev1.ExpenseStatus_EXPENSE_STATUS_CONFIRMED:   "подтверждена",
		corev1.ExpenseStatus_EXPENSE_STATUS_CANCELLED:   "отменена",
		corev1.ExpenseStatus_EXPENSE_STATUS_UNSPECIFIED: "не указан",
	}
	for status, want := range expenseCases {
		if got := exportExpenseStatusName(status); got != want {
			t.Errorf("exportExpenseStatusName(%v) = %q, want %q", status, got, want)
		}
	}

	settlementCases := map[corev1.SettlementStatus]string{
		corev1.SettlementStatus_SETTLEMENT_STATUS_PENDING:     "ожидает подтверждения",
		corev1.SettlementStatus_SETTLEMENT_STATUS_CONFIRMED:   "подтверждён",
		corev1.SettlementStatus_SETTLEMENT_STATUS_CANCELLED:   "отменён",
		corev1.SettlementStatus_SETTLEMENT_STATUS_UNSPECIFIED: "не указан",
	}
	for status, want := range settlementCases {
		if got := exportSettlementStatusName(status); got != want {
			t.Errorf("exportSettlementStatusName(%v) = %q, want %q", status, got, want)
		}
	}
}

func TestExportRowTextIsRussian(t *testing.T) {
	if got, want := exportUserName(&corev1.GroupMember{UserId: 42}), "Пользователь 42"; got != want {
		t.Errorf("fallback user name = %q, want %q", got, want)
	}
	if got, want := exportAdjustmentDescription(corev1.AdjustmentType_ADJUSTMENT_TYPE_CORRECTION, "Обед"), "Корректировка: Обед"; got != want {
		t.Errorf("correction description = %q, want %q", got, want)
	}
	if got, want := exportAdjustmentDescription(corev1.AdjustmentType_ADJUSTMENT_TYPE_REFUND, ""), "Возврат"; got != want {
		t.Errorf("refund description = %q, want %q", got, want)
	}
	if got, want := exportExpenseNote(12, corev1.ExpenseStatus_EXPENSE_STATUS_PENDING), "Расход №12, статус: ожидает подтверждения"; got != want {
		t.Errorf("expense note = %q, want %q", got, want)
	}
	if got, want := exportAdjustmentNote(4, 12, corev1.AdjustmentType_ADJUSTMENT_TYPE_CORRECTION), "Корректировка №4 к расходу №12"; got != want {
		t.Errorf("adjustment note = %q, want %q", got, want)
	}
	if got, want := exportAdjustmentNote(5, 12, corev1.AdjustmentType_ADJUSTMENT_TYPE_REFUND), "Возврат №5 к расходу №12"; got != want {
		t.Errorf("refund note = %q, want %q", got, want)
	}
	if got, want := exportSettlementNote(7, "Иван", corev1.SettlementStatus_SETTLEMENT_STATUS_CONFIRMED), "Расчёт №7: получатель Иван, статус: подтверждён"; got != want {
		t.Errorf("settlement note = %q, want %q", got, want)
	}
}
