package http

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	corev1 "delim/pkg/gen/core/v1"
	documentv1 "delim/pkg/gen/document/v1"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
)

const multipartOverheadLimit = 1 << 20

var allowedReceiptContentTypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

type receiptCoreClient interface {
	GetGroup(context.Context, *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error)
}

type documentClient interface {
	healthChecker
	CreateReceipt(context.Context, *documentv1.CreateReceiptRequest) (*documentv1.CreateReceiptResponse, error)
	GetReceipt(context.Context, *documentv1.GetReceiptRequest) (*documentv1.GetReceiptResponse, error)
	GetDocumentJob(context.Context, *documentv1.GetDocumentJobRequest) (*documentv1.GetDocumentJobResponse, error)
	GetOCRResult(context.Context, *documentv1.GetOCRResultRequest) (*documentv1.GetOCRResultResponse, error)
	RetryReceiptOCR(context.Context, *documentv1.RetryReceiptOCRRequest) (*documentv1.RetryReceiptOCRResponse, error)
	DeleteReceipt(context.Context, *documentv1.DeleteReceiptRequest) (*documentv1.DeleteReceiptResponse, error)
	CreateExport(context.Context, *documentv1.CreateExportRequest) (*documentv1.CreateExportResponse, error)
	GetExport(context.Context, *documentv1.GetExportRequest) (*documentv1.GetExportResponse, error)
	DownloadExport(context.Context, *documentv1.DownloadExportRequest) (grpc.ServerStreamingClient[documentv1.DownloadExportChunk], error)
}

type createReceiptResponse struct {
	Receipt receiptResponse     `json:"receipt"`
	Job     documentJobResponse `json:"job"`
}

type receiptResponse struct {
	ID          int64     `json:"id"`
	GroupID     int64     `json:"group_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type documentJobResponse struct {
	ID         int64      `json:"id"`
	ReceiptID  int64      `json:"receipt_id"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	ErrorCode  string     `json:"error_code,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type ocrResultResponse struct {
	Status     string            `json:"status"`
	Merchant   *string           `json:"merchant,omitempty"`
	Date       *time.Time        `json:"date,omitempty"`
	TotalMinor *int64            `json:"total_minor,omitempty"`
	Currency   *string           `json:"currency,omitempty"`
	Items      []ocrItemResponse `json:"items"`
	Confidence float32           `json:"confidence"`
	QRFound    bool              `json:"qr_found"`
}

type ocrItemResponse struct {
	Name           string  `json:"name"`
	Quantity       *string `json:"quantity,omitempty"`
	UnitPriceMinor *int64  `json:"unit_price_minor,omitempty"`
	AmountMinor    int64   `json:"amount_minor"`
	Confidence     float32 `json:"confidence"`
}

type receiptUpload struct {
	filename    string
	contentType string
	content     []byte
}

func createReceipt(core receiptCoreClient, document documentClient, uploadMaxBytes int64) http.HandlerFunc {
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
		if uploadMaxBytes <= 0 {
			writeError(w, http.StatusServiceUnavailable, "upload_not_configured", "receipt upload is not configured")
			return
		}

		if _, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: groupID}); err != nil {
			writeDownstreamError(w, err)
			return
		}
		upload, err := readReceiptUpload(w, r, uploadMaxBytes)
		if err != nil {
			if errors.Is(err, errUploadTooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "receipt image is too large")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid_upload", err.Error())
			return
		}
		response, err := document.CreateReceipt(r.Context(), &documentv1.CreateReceiptRequest{
			ActorUserId: actorID,
			GroupId:     groupID,
			Filename:    upload.filename,
			ContentType: upload.contentType,
			Content:     upload.content,
		})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, createReceiptResponse{
			Receipt: receiptToResponse(response.GetReceipt()),
			Job:     documentJobToResponse(response.GetJob()),
		})
	}
}

var errUploadTooLarge = errors.New("receipt upload is too large")

func readReceiptUpload(w http.ResponseWriter, r *http.Request, maxBytes int64) (receiptUpload, error) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return receiptUpload{}, errors.New("content type must be multipart/form-data")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+multipartOverheadLimit)
	reader := multipart.NewReader(r.Body, params["boundary"])
	part, err := reader.NextPart()
	if err != nil {
		return receiptUpload{}, errors.New("exactly one file is required")
	}
	defer part.Close()
	if part.FormName() != "file" || part.FileName() == "" {
		return receiptUpload{}, errors.New("multipart field file is required")
	}
	contentType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
	if err != nil {
		return receiptUpload{}, errors.New("invalid receipt content type")
	}
	if _, ok := allowedReceiptContentTypes[contentType]; !ok {
		return receiptUpload{}, errors.New("unsupported receipt content type")
	}
	content, err := io.ReadAll(io.LimitReader(part, maxBytes+1))
	if err != nil {
		return receiptUpload{}, errors.New("cannot read receipt image")
	}
	if int64(len(content)) > maxBytes {
		return receiptUpload{}, errUploadTooLarge
	}
	if len(content) == 0 {
		return receiptUpload{}, errors.New("receipt image is empty")
	}
	if next, err := reader.NextPart(); err != io.EOF {
		if err == nil {
			_ = next.Close()
		}
		return receiptUpload{}, errors.New("exactly one file is required")
	}
	filename := filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
	if filename == "." || filename == "" {
		return receiptUpload{}, errors.New("receipt filename is required")
	}
	return receiptUpload{filename: filename, contentType: contentType, content: content}, nil
}

func getReceipt(document documentClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, resourceID, ok := documentResourceIDs(w, r, "receiptID", "receipt")
		if !ok {
			return
		}
		response, err := document.GetReceipt(r.Context(), &documentv1.GetReceiptRequest{ActorUserId: actorID, ReceiptId: resourceID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, receiptToResponse(response.GetReceipt()))
	}
}

func getDocumentJob(document documentClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, resourceID, ok := documentResourceIDs(w, r, "jobID", "document job")
		if !ok {
			return
		}
		response, err := document.GetDocumentJob(r.Context(), &documentv1.GetDocumentJobRequest{ActorUserId: actorID, JobId: resourceID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, documentJobToResponse(response.GetJob()))
	}
}

func getOCRResult(document documentClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, resourceID, ok := documentResourceIDs(w, r, "receiptID", "receipt")
		if !ok {
			return
		}
		response, err := document.GetOCRResult(r.Context(), &documentv1.GetOCRResultRequest{ActorUserId: actorID, ReceiptId: resourceID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, ocrResultToResponse(response))
	}
}

func retryReceiptOCR(document documentClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, resourceID, ok := documentResourceIDs(w, r, "receiptID", "receipt")
		if !ok {
			return
		}
		response, err := document.RetryReceiptOCR(r.Context(), &documentv1.RetryReceiptOCRRequest{ActorUserId: actorID, ReceiptId: resourceID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, documentJobToResponse(response.GetJob()))
	}
}

func deleteReceipt(document documentClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, resourceID, ok := documentResourceIDs(w, r, "receiptID", "receipt")
		if !ok {
			return
		}
		if _, err := document.DeleteReceipt(r.Context(), &documentv1.DeleteReceiptRequest{ActorUserId: actorID, ReceiptId: resourceID}); err != nil {
			writeDownstreamError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func documentResourceIDs(w http.ResponseWriter, r *http.Request, parameter, resource string) (int64, int64, bool) {
	actorID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
		return 0, 0, false
	}
	resourceID, err := parseID(chi.URLParam(r, parameter))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", "invalid "+resource+" id")
		return 0, 0, false
	}
	return actorID, resourceID, true
}

func receiptToResponse(receipt *documentv1.Receipt) receiptResponse {
	if receipt == nil {
		return receiptResponse{}
	}
	return receiptResponse{
		ID: receipt.Id, GroupID: receipt.GroupId, Filename: receipt.Filename,
		ContentType: receipt.ContentType, SizeBytes: receipt.SizeBytes,
		Status: receiptStatusName(receipt.Status), CreatedAt: receipt.CreatedAt.AsTime(),
	}
}

func documentJobToResponse(job *documentv1.DocumentJob) documentJobResponse {
	if job == nil {
		return documentJobResponse{}
	}
	response := documentJobResponse{
		ID: job.Id, ReceiptID: job.ReceiptId, Type: documentJobTypeName(job.Type),
		Status: documentJobStatusName(job.Status), ErrorCode: job.ErrorCode,
		CreatedAt: job.CreatedAt.AsTime(),
	}
	if job.StartedAt != nil {
		value := job.StartedAt.AsTime()
		response.StartedAt = &value
	}
	if job.FinishedAt != nil {
		value := job.FinishedAt.AsTime()
		response.FinishedAt = &value
	}
	return response
}

func ocrResultToResponse(result *documentv1.GetOCRResultResponse) ocrResultResponse {
	response := ocrResultResponse{
		Status: receiptStatusName(result.GetStatus()), Items: make([]ocrItemResponse, 0, len(result.GetItems())),
		Confidence: result.GetConfidence(), QRFound: result.GetQrFound(),
	}
	response.Merchant = result.Merchant
	response.TotalMinor = result.TotalMinor
	response.Currency = result.Currency
	if result.Date != nil {
		value := result.Date.AsTime()
		response.Date = &value
	}
	for _, item := range result.GetItems() {
		response.Items = append(response.Items, ocrItemResponse{
			Name: item.Name, Quantity: item.Quantity, UnitPriceMinor: item.UnitPriceMinor,
			AmountMinor: item.AmountMinor, Confidence: item.Confidence,
		})
	}
	return response
}

func receiptStatusName(value documentv1.ReceiptStatus) string {
	switch value {
	case documentv1.ReceiptStatus_RECEIPT_STATUS_UPLOADED:
		return "uploaded"
	case documentv1.ReceiptStatus_RECEIPT_STATUS_QUEUED:
		return "queued"
	case documentv1.ReceiptStatus_RECEIPT_STATUS_PROCESSING:
		return "processing"
	case documentv1.ReceiptStatus_RECEIPT_STATUS_READY:
		return "ready"
	case documentv1.ReceiptStatus_RECEIPT_STATUS_FAILED:
		return "failed"
	case documentv1.ReceiptStatus_RECEIPT_STATUS_DELETED:
		return "deleted"
	default:
		return "unspecified"
	}
}

func documentJobTypeName(value documentv1.DocumentJobType) string {
	if value == documentv1.DocumentJobType_DOCUMENT_JOB_TYPE_OCR {
		return "ocr"
	}
	return "unspecified"
}

func documentJobStatusName(value documentv1.DocumentJobStatus) string {
	switch value {
	case documentv1.DocumentJobStatus_DOCUMENT_JOB_STATUS_PENDING:
		return "pending"
	case documentv1.DocumentJobStatus_DOCUMENT_JOB_STATUS_PROCESSING:
		return "processing"
	case documentv1.DocumentJobStatus_DOCUMENT_JOB_STATUS_COMPLETED:
		return "completed"
	case documentv1.DocumentJobStatus_DOCUMENT_JOB_STATUS_FAILED:
		return "failed"
	default:
		return "unspecified"
	}
}
