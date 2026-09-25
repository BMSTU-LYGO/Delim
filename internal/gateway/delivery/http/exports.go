package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/exportcap"
	"delim/internal/gateway/repository/postgres"
	corev1 "delim/pkg/gen/core/v1"
	documentv1 "delim/pkg/gen/document/v1"
	"delim/pkg/maxapi"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type exportCoreClient interface {
	GetGroup(context.Context, *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error)
	ListGroupMembers(context.Context, *corev1.ListGroupMembersRequest) (*corev1.ListGroupMembersResponse, error)
	ListExpenses(context.Context, *corev1.ListExpensesRequest) (*corev1.ListExpensesResponse, error)
	ListSettlements(context.Context, *corev1.ListSettlementsRequest) (*corev1.ListSettlementsResponse, error)
	ListAdjustments(context.Context, *corev1.ListAdjustmentsRequest) (*corev1.ListAdjustmentsResponse, error)
	ListGroupAdjustments(context.Context, *corev1.ListGroupAdjustmentsRequest) (*corev1.ListAdjustmentsResponse, error)
}

type personalExportStore interface {
	ListActivePersonalSubscriptions(context.Context, []int64) ([]postgres.PersonalSubscription, error)
}

type exportFileSender interface {
	UploadFile(context.Context, string, string, io.Reader) (string, error)
	SendMessage(context.Context, int64, maxapi.NewMessage) (maxapi.Message, error)
}

type createExportRequest struct {
	Format string `json:"format"`
}

type exportResponse struct {
	ID          int64      `json:"id"`
	GroupID     int64      `json:"group_id"`
	Format      string     `json:"format"`
	Status      string     `json:"status"`
	Filename    string     `json:"filename"`
	ErrorCode   string     `json:"error_code,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	DownloadURL string     `json:"download_url,omitempty"`
}

const exportDownloadTTL = 10 * time.Minute

func createExport(core exportCoreClient, document documentClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		groupID, err := parseID(chi.URLParam(r, "groupID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		var request createExportRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		exportFormat, ok := exportFormatFromName(request.Format)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_argument", "unsupported export format")
			return
		}

		group, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		if group.GetGroup() == nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		rows, err := buildExportRows(r.Context(), core, actorID, groupID)
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		response, err := document.CreateExport(r.Context(), &documentv1.CreateExportRequest{
			ActorUserId: actorID,
			GroupId:     groupID,
			GroupName:   group.GetGroup().GetName(),
			Format:      exportFormat,
			Rows:        rows,
		})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, exportToResponse(response.GetExport()))
	}
}

func getExport(core exportCoreClient, document documentClient, capabilities *exportcap.Manager, miniAppURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, exportID, ok := documentResourceIDs(w, r, "exportID", "export")
		if !ok {
			return
		}
		record, ok := authorizedExport(w, r, core, document, actorID, exportID)
		if !ok {
			return
		}
		response := exportToResponse(record)
		if record.GetStatus() == documentv1.ExportStatus_EXPORT_STATUS_READY {
			token, err := capabilities.Issue(exportID, actorID, exportDownloadTTL)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}
			response.DownloadURL = exportDownloadURL(r, miniAppURL, exportID, token)
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func downloadExport(core exportCoreClient, document documentClient, sessions *auth.Manager, capabilities *exportcap.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exportID, err := parseID(chi.URLParam(r, "exportID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid export id")
			return
		}
		actorID, ok := downloadActor(r, sessions, capabilities, exportID)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_download_capability", "invalid or expired download capability")
			return
		}
		record, ok := authorizedExport(w, r, core, document, actorID, exportID)
		if !ok {
			return
		}
		if record.GetStatus() != documentv1.ExportStatus_EXPORT_STATUS_READY {
			writeError(w, http.StatusConflict, "export_not_ready", "export is not ready")
			return
		}
		stream, err := document.DownloadExport(r.Context(), &documentv1.DownloadExportRequest{ActorUserId: actorID, ExportId: exportID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		defer stream.CloseSend()
		first, err := stream.Recv()
		if err != nil && err != io.EOF {
			writeDownstreamError(w, err)
			return
		}

		w.Header().Set("Content-Type", exportContentType(record.GetFormat()))
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": record.GetFilename()}))
		w.WriteHeader(http.StatusOK)
		if err == io.EOF {
			return
		}
		if _, err := w.Write(first.GetContent()); err != nil {
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
			if _, err := w.Write(chunk.GetContent()); err != nil {
				return
			}
		}
	}
}

func sendExport(core exportCoreClient, document documentClient, subscriptions personalExportStore, maxAPI exportFileSender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, exportID, ok := documentResourceIDs(w, r, "exportID", "export")
		if !ok {
			return
		}
		maxUserID, ok := maxUserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		record, ok := authorizedExport(w, r, core, document, actorID, exportID)
		if !ok {
			return
		}
		if record.GetStatus() != documentv1.ExportStatus_EXPORT_STATUS_READY {
			writeError(w, http.StatusConflict, "export_not_ready", "export is not ready")
			return
		}
		personal, err := subscriptions.ListActivePersonalSubscriptions(r.Context(), []int64{maxUserID})
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "subscription_unavailable", "MAX notifications are temporarily unavailable")
			return
		}
		if len(personal) != 1 || personal[0].MAXUserID != maxUserID || personal[0].ChatID == 0 {
			writeError(w, http.StatusConflict, "personal_subscription_required", "connect the MAX bot to receive exports")
			return
		}
		stream, err := document.DownloadExport(r.Context(), &documentv1.DownloadExportRequest{ActorUserId: actorID, ExportId: exportID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		defer stream.CloseSend()
		token, err := maxAPI.UploadFile(r.Context(), record.GetFilename(), exportContentType(record.GetFormat()), &exportStreamReader{stream: stream})
		if err != nil {
			writeError(w, http.StatusBadGateway, "export_delivery_failed", "could not deliver the export to MAX")
			return
		}
		if err := sendExportMessage(r.Context(), maxAPI, personal[0].ChatID, maxapi.NewMessage{
			Text:        "Экспорт группы готов.",
			Attachments: []any{maxapi.FileAttachment{Type: "file", Payload: maxapi.FileAttachmentPayload{Token: token}}},
		}); err != nil {
			writeError(w, http.StatusBadGateway, "export_delivery_failed", "could not deliver the export to MAX")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"delivered": true})
	}
}

func sendExportMessage(ctx context.Context, sender exportFileSender, chatID int64, message maxapi.NewMessage) error {
	for attempt := 0; attempt < 3; attempt++ {
		if _, err := sender.SendMessage(ctx, chatID, message); err != nil {
			var apiErr *maxapi.APIError
			if attempt == 2 || !errors.As(err, &apiErr) || apiErr.Code != "attachment.not.ready" {
				return err
			}
			timer := time.NewTimer(time.Duration(1<<attempt) * 250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		return nil
	}
	return nil
}

type exportChunkStream interface {
	Recv() (*documentv1.DownloadExportChunk, error)
	CloseSend() error
}

type exportStreamReader struct {
	stream  exportChunkStream
	pending []byte
}

func (r *exportStreamReader) Read(target []byte) (int, error) {
	for len(r.pending) == 0 {
		chunk, err := r.stream.Recv()
		if err != nil {
			return 0, err
		}
		r.pending = chunk.GetContent()
	}
	n := copy(target, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func downloadActor(r *http.Request, sessions *auth.Manager, capabilities *exportcap.Manager, exportID int64) (int64, bool) {
	const prefix = "Bearer "
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, prefix) && len(header) > len(prefix) && !strings.Contains(header[len(prefix):], " ") {
		session, err := sessions.Verify(header[len(prefix):])
		if err == nil {
			return session.UserID, true
		}
	}
	capExportID, actorID, err := capabilities.Verify(r.URL.Query().Get("t"))
	return actorID, err == nil && capExportID == exportID
}

func exportDownloadURL(r *http.Request, miniAppURL string, exportID int64, token string) string {
	origin := ""
	if parsed, err := url.Parse(miniAppURL); err == nil && parsed.Scheme == "https" && parsed.Host != "" {
		origin = "https://" + parsed.Host
	}
	if origin == "" {
		origin = "https://" + r.Host
	}
	return origin + "/api/v1/exports/" + strconv.FormatInt(exportID, 10) + "/download?t=" + url.QueryEscape(token)
}

func authorizedExport(w http.ResponseWriter, r *http.Request, core exportCoreClient, document documentClient, actorID, exportID int64) (*documentv1.Export, bool) {
	response, err := document.GetExport(r.Context(), &documentv1.GetExportRequest{ActorUserId: actorID, ExportId: exportID})
	if err != nil {
		writeDownstreamError(w, err)
		return nil, false
	}
	record := response.GetExport()
	if record == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return nil, false
	}
	if _, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: record.GetGroupId()}); err != nil {
		writeDownstreamError(w, err)
		return nil, false
	}
	return record, true
}

func buildExportRows(ctx context.Context, core exportCoreClient, actorID, groupID int64) ([]*documentv1.ExportReportRow, error) {
	members, err := core.ListGroupMembers(ctx, &corev1.ListGroupMembersRequest{ActorUserId: actorID, GroupId: groupID})
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(members.GetMembers()))
	for _, member := range members.GetMembers() {
		names[member.GetUserId()] = exportUserName(member)
	}

	expenses, err := listAllExpenses(ctx, core, actorID, groupID)
	if err != nil {
		return nil, err
	}
	// Fetch every adjustment for the group in a single call to avoid a
	// per-expense round trip; the export is assembled synchronously in the
	// CreateExport HTTP handler, so this directly bounds its latency.
	groupAdjustments, err := core.ListGroupAdjustments(ctx, &corev1.ListGroupAdjustmentsRequest{ActorUserId: actorID, GroupId: groupID})
	if err != nil {
		return nil, err
	}
	adjustmentsByExpense := make(map[int64][]*corev1.Adjustment)
	for _, adjustment := range groupAdjustments.GetAdjustments() {
		adjustmentsByExpense[adjustment.GetExpenseId()] = append(adjustmentsByExpense[adjustment.GetExpenseId()], adjustment)
	}
	rows := make([]*documentv1.ExportReportRow, 0, len(expenses))
	for _, expense := range expenses {
		rows = append(rows, &documentv1.ExportReportRow{
			Date: expense.GetExpenseDate(), Description: expense.GetDescription(),
			Payer: names[expense.GetPayerUserId()], AmountMinor: expense.GetAmountMinor(),
			Currency: expense.GetCurrency(), Note: exportExpenseNote(expense.GetId(), expense.GetStatus()),
		})
		for _, adjustment := range adjustmentsByExpense[expense.GetId()] {
			rows = append(rows, &documentv1.ExportReportRow{
				Date: adjustment.GetCreatedAt(), Description: exportAdjustmentDescription(adjustment.GetType(), expense.GetDescription()),
				Payer: names[adjustment.GetCreatedBy()], AmountMinor: adjustment.GetAmountMinor(),
				Currency: adjustment.GetCurrency(), Note: exportAdjustmentNote(adjustment.GetId(), expense.GetId(), adjustment.GetType()),
			})
		}
	}

	settlements, err := listAllSettlements(ctx, core, actorID, groupID)
	if err != nil {
		return nil, err
	}
	for _, settlement := range settlements {
		rows = append(rows, &documentv1.ExportReportRow{
			Date: settlement.GetCreatedAt(), Description: "Расчёт",
			Payer: names[settlement.GetSenderUserId()], AmountMinor: settlement.GetAmountMinor(),
			Currency: settlement.GetCurrency(), Note: exportSettlementNote(settlement.GetId(), names[settlement.GetReceiverUserId()], settlement.GetStatus()),
		})
	}
	return rows, nil
}

func listAllExpenses(ctx context.Context, core exportCoreClient, actorID, groupID int64) ([]*corev1.Expense, error) {
	var result []*corev1.Expense
	var cursor int64
	for {
		response, err := core.ListExpenses(ctx, &corev1.ListExpensesRequest{ActorUserId: actorID, GroupId: groupID, Page: &corev1.PageRequest{Limit: maxPageLimit, CursorId: cursor}})
		if err != nil {
			return nil, err
		}
		result = append(result, response.GetExpenses()...)
		next := response.GetPage().GetNextCursorId()
		if next == 0 {
			return result, nil
		}
		if next == cursor {
			return nil, status.Error(codes.Internal, "invalid expense pagination cursor")
		}
		cursor = next
	}
}

func listAllSettlements(ctx context.Context, core exportCoreClient, actorID, groupID int64) ([]*corev1.Settlement, error) {
	var result []*corev1.Settlement
	var cursor int64
	for {
		response, err := core.ListSettlements(ctx, &corev1.ListSettlementsRequest{ActorUserId: actorID, GroupId: groupID, Page: &corev1.PageRequest{Limit: maxPageLimit, CursorId: cursor}})
		if err != nil {
			return nil, err
		}
		result = append(result, response.GetSettlements()...)
		next := response.GetPage().GetNextCursorId()
		if next == 0 {
			return result, nil
		}
		if next == cursor {
			return nil, status.Error(codes.Internal, "invalid settlement pagination cursor")
		}
		cursor = next
	}
}

func exportUserName(member *corev1.GroupMember) string {
	user := member.GetUser()
	if user != nil {
		if name := strings.TrimSpace(user.GetFirstName() + " " + user.GetLastName()); name != "" {
			return name
		}
		if user.GetUsername() != "" {
			return "@" + user.GetUsername()
		}
	}
	return "Пользователь " + strconv.FormatInt(member.GetUserId(), 10)
}

func exportAdjustmentDescription(value corev1.AdjustmentType, expenseDescription string) string {
	label := exportAdjustmentTypeName(value)
	if expenseDescription == "" {
		return label
	}
	return label + ": " + expenseDescription
}

func exportExpenseNote(expenseID int64, value corev1.ExpenseStatus) string {
	return fmt.Sprintf("Расход №%d, статус: %s", expenseID, exportExpenseStatusName(value))
}

func exportAdjustmentNote(adjustmentID, expenseID int64, value corev1.AdjustmentType) string {
	return fmt.Sprintf("%s №%d к расходу №%d", exportAdjustmentTypeName(value), adjustmentID, expenseID)
}

func exportSettlementNote(settlementID int64, receiver string, value corev1.SettlementStatus) string {
	return fmt.Sprintf("Расчёт №%d: получатель %s, статус: %s", settlementID, receiver, exportSettlementStatusName(value))
}

func exportAdjustmentTypeName(value corev1.AdjustmentType) string {
	if value == corev1.AdjustmentType_ADJUSTMENT_TYPE_REFUND {
		return "Возврат"
	}
	return "Корректировка"
}

func exportExpenseStatusName(value corev1.ExpenseStatus) string {
	switch value {
	case corev1.ExpenseStatus_EXPENSE_STATUS_PENDING:
		return "ожидает подтверждения"
	case corev1.ExpenseStatus_EXPENSE_STATUS_CONFIRMED:
		return "подтверждена"
	case corev1.ExpenseStatus_EXPENSE_STATUS_CANCELLED:
		return "отменена"
	default:
		return "не указан"
	}
}

func exportSettlementStatusName(value corev1.SettlementStatus) string {
	switch value {
	case corev1.SettlementStatus_SETTLEMENT_STATUS_PENDING:
		return "ожидает подтверждения"
	case corev1.SettlementStatus_SETTLEMENT_STATUS_CONFIRMED:
		return "подтверждён"
	case corev1.SettlementStatus_SETTLEMENT_STATUS_CANCELLED:
		return "отменён"
	default:
		return "не указан"
	}
}

func exportFormatFromName(value string) (documentv1.ExportFormat, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "csv":
		return documentv1.ExportFormat_EXPORT_FORMAT_CSV, true
	case "pdf":
		return documentv1.ExportFormat_EXPORT_FORMAT_PDF, true
	case "xlsx":
		return documentv1.ExportFormat_EXPORT_FORMAT_XLSX, true
	default:
		return documentv1.ExportFormat_EXPORT_FORMAT_UNSPECIFIED, false
	}
}

func exportToResponse(value *documentv1.Export) exportResponse {
	if value == nil {
		return exportResponse{}
	}
	response := exportResponse{
		ID: value.Id, GroupID: value.GroupId, Format: exportFormatName(value.Format), Status: exportStatusName(value.Status),
		Filename: value.Filename, ErrorCode: value.ErrorCode, CreatedAt: value.CreatedAt.AsTime(),
	}
	if value.FinishedAt != nil {
		finishedAt := value.FinishedAt.AsTime()
		response.FinishedAt = &finishedAt
	}
	return response
}

func exportFormatName(value documentv1.ExportFormat) string {
	switch value {
	case documentv1.ExportFormat_EXPORT_FORMAT_CSV:
		return "csv"
	case documentv1.ExportFormat_EXPORT_FORMAT_PDF:
		return "pdf"
	case documentv1.ExportFormat_EXPORT_FORMAT_XLSX:
		return "xlsx"
	default:
		return "unspecified"
	}
}

func exportStatusName(value documentv1.ExportStatus) string {
	switch value {
	case documentv1.ExportStatus_EXPORT_STATUS_PENDING:
		return "pending"
	case documentv1.ExportStatus_EXPORT_STATUS_PROCESSING:
		return "processing"
	case documentv1.ExportStatus_EXPORT_STATUS_READY:
		return "ready"
	case documentv1.ExportStatus_EXPORT_STATUS_FAILED:
		return "failed"
	default:
		return "unspecified"
	}
}

func exportContentType(value documentv1.ExportFormat) string {
	switch value {
	case documentv1.ExportFormat_EXPORT_FORMAT_PDF:
		return "application/pdf"
	case documentv1.ExportFormat_EXPORT_FORMAT_XLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "text/csv; charset=utf-8"
	}
}
