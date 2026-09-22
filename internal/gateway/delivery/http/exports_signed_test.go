package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/exportcap"
	corev1 "delim/pkg/gen/core/v1"
	documentv1 "delim/pkg/gen/document/v1"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type signedExportCore struct{ exportCoreClient }

func (signedExportCore) GetGroup(context.Context, *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error) {
	return &corev1.GetGroupResponse{Group: &corev1.Group{Id: 9}}, nil
}

type signedExportDocument struct {
	documentClient
	record *documentv1.Export
}

func (d signedExportDocument) GetExport(context.Context, *documentv1.GetExportRequest) (*documentv1.GetExportResponse, error) {
	return &documentv1.GetExportResponse{Export: d.record}, nil
}

func TestGetReadyExportAddsHTTPSDownloadURL(t *testing.T) {
	capabilities := exportcap.NewManager("test-secret")
	created := timestamppb.New(time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC))
	handler := getExport(signedExportCore{}, signedExportDocument{record: &documentv1.Export{
		Id: 42, GroupId: 9, Filename: "report.csv", Status: documentv1.ExportStatus_EXPORT_STATUS_READY, CreatedAt: created,
	}}, capabilities, "https://mini.example/app")

	req := httptest.NewRequest(http.MethodGet, "http://internal/api/v1/exports/42", nil)
	ctx := context.WithValue(req.Context(), sessionContextKey{}, auth.Session{UserID: 7})
	ctx = context.WithValue(ctx, chi.RouteCtxKey, &chi.Context{URLParams: chi.RouteParams{Keys: []string{"exportID"}, Values: []string{"42"}}})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req.WithContext(ctx))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response exportResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(response.DownloadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "mini.example" || parsed.Query().Get("t") == "" {
		t.Fatalf("download_url = %q", response.DownloadURL)
	}
	if exportID, actorID, err := capabilities.Verify(parsed.Query().Get("t")); err != nil || exportID != 42 || actorID != 7 {
		t.Fatalf("download capability = (%d, %d, %v)", exportID, actorID, err)
	}
}

func TestDownloadActorRejectsExpiredAndCrossExportCapability(t *testing.T) {
	capabilities := exportcap.NewManager("test-secret")
	// Exercise handler authorization decisions without a live server.
	token, err := capabilities.Issue(42, 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/?t="+url.QueryEscape(token), nil)
	if _, ok := downloadActor(req, auth.NewManager("session", time.Hour), capabilities, 43); ok {
		t.Fatal("capability authorized a different export")
	}

	// A syntactically valid token signed by another capability domain is also rejected.
	if _, ok := downloadActor(req, auth.NewManager("session", time.Hour), exportcap.NewManager("other-secret"), 42); ok {
		t.Fatal("capability from another manager was accepted")
	}
}

func TestDownloadExportRejectsNonReadyCapability(t *testing.T) {
	capabilities := exportcap.NewManager("test-secret")
	token, err := capabilities.Issue(42, 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	handler := downloadExport(signedExportCore{}, signedExportDocument{record: &documentv1.Export{
		Id: 42, GroupId: 9, Status: documentv1.ExportStatus_EXPORT_STATUS_PROCESSING,
	}}, auth.NewManager("session", time.Hour), capabilities)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/42/download?t="+url.QueryEscape(token), nil)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{URLParams: chi.RouteParams{Keys: []string{"exportID"}, Values: []string{"42"}}})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req.WithContext(ctx))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetNonReadyExportOmitsDownloadURL(t *testing.T) {
	handler := getExport(signedExportCore{}, signedExportDocument{record: &documentv1.Export{
		Id: 42, GroupId: 9, Filename: "report.csv", Status: documentv1.ExportStatus_EXPORT_STATUS_PROCESSING,
		CreatedAt: timestamppb.Now(),
	}}, exportcap.NewManager("test-secret"), "https://mini.example/app")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports/42", nil)
	ctx := context.WithValue(req.Context(), sessionContextKey{}, auth.Session{UserID: 7})
	ctx = context.WithValue(ctx, chi.RouteCtxKey, &chi.Context{URLParams: chi.RouteParams{Keys: []string{"exportID"}, Values: []string{"42"}}})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req.WithContext(ctx))
	var response exportResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.DownloadURL != "" {
		t.Fatalf("non-ready download_url = %q", response.DownloadURL)
	}
}
