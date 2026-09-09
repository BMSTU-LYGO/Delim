package grpcx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"delim/pkg/metricsx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestExtractRequestIDReturnsFirstIncomingMetadataValue(t *testing.T) {
	t.Parallel()

	md := metadata.New(map[string]string{"x-request-id": "abc-123"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	if got := extractRequestID(ctx); got != "abc-123" {
		t.Fatalf("extractRequestID = %q, want %q", got, "abc-123")
	}
}

func TestExtractRequestIDReturnsEmptyWhenMetadataMissing(t *testing.T) {
	t.Parallel()

	if got := extractRequestID(context.Background()); got != "" {
		t.Fatalf("extractRequestID = %q, want empty", got)
	}
}

func TestRequestIDFromContextReturnsStoredValue(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), requestIDContextKey{}, "ctx-id")
	if got := RequestIDFromContext(ctx); got != "ctx-id" {
		t.Fatalf("RequestIDFromContext = %q, want %q", got, "ctx-id")
	}
}

func TestRequestIDFromContextEmptyOnPlainContext(t *testing.T) {
	t.Parallel()

	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("RequestIDFromContext = %q, want empty", got)
	}
}

func TestErrorClassReturnsBoundedValues(t *testing.T) {
	t.Parallel()

	if got := ErrorClass(nil); got != "ok" {
		t.Fatalf("ErrorClass(nil) = %q, want ok", got)
	}
	if got := ErrorClass(status.Error(codes.NotFound, "secret-do-not-log")); got != "grpc_NOT_FOUND" {
		t.Fatalf("ErrorClass(grpc) = %q, want grpc_NOT_FOUND", got)
	}
	if got := ErrorClass(context.DeadlineExceeded); got != "deadline_exceeded" {
		t.Fatalf("ErrorClass(deadline) = %q, want deadline_exceeded", got)
	}
	if got := ErrorClass(context.Canceled); got != "canceled" {
		t.Fatalf("ErrorClass(canceled) = %q, want canceled", got)
	}
	if got := ErrorClass(errors.New("boom")); got != "internal" {
		t.Fatalf("ErrorClass(generic) = %q, want internal", got)
	}
}

func TestErrorClassNeverLeaksErrorMessage(t *testing.T) {
	t.Parallel()

	secret := "super-secret-token"
	class := ErrorClass(errors.New(secret))
	if strings.Contains(class, secret) {
		t.Fatalf("ErrorClass leaked message: %q", class)
	}
}

func TestUnaryInterceptorEmitsNormalizedCompletionRecord(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil)).With("service", "core")

	md := metadata.New(map[string]string{"x-request-id": "trace-1"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		if got := RequestIDFromContext(ctx); got != "trace-1" {
			t.Fatalf("handler request_id = %q, want %q", got, "trace-1")
		}
		return "ok", nil
	}

	if _, err := UnaryRequestIDInterceptor(logger)(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"},
		handler,
	); err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	entries := decodeLogEntries(t, logBuf.String())
	completed := findEntry(entries, "grpc request completed")
	if completed == nil {
		t.Fatalf("expected completed entry, got %v", entries)
	}
	assertNormalizedFields(t, completed, "core", "trace-1", "/test.Service/Method", "success", "success")
	assertDurationNumeric(t, completed)
	if _, ok := completed["error_class"]; ok {
		t.Fatalf("success record must not include error_class, got %v", completed)
	}
}

func TestUnaryInterceptorEmitsNormalizedErrorRecord(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil)).With("service", "core")

	md := metadata.New(map[string]string{"x-request-id": "trace-err"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	secret := "MAX_BOT_TOKEN=should-not-leak"
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.Unauthenticated, secret)
	}

	if _, err := UnaryRequestIDInterceptor(logger)(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"},
		handler,
	); err == nil {
		t.Fatal("expected error")
	}

	entries := decodeLogEntries(t, logBuf.String())
	failed := findEntry(entries, "grpc request failed")
	if failed == nil {
		t.Fatalf("expected failed entry, got %v", entries)
	}
	assertNormalizedFields(t, failed, "core", "trace-err", "/test.Service/Method", "error", "error")
	assertDurationNumeric(t, failed)
	if class, _ := failed["error_class"].(string); class != "grpc_UNAUTHENTICATED" {
		t.Fatalf("error_class = %q, want grpc_UNAUTHENTICATED", class)
	}
	if raw, ok := failed["error"].(string); ok && strings.Contains(raw, secret) {
		t.Fatalf("error field leaked secret: %q", raw)
	}
	if strings.Contains(logBuf.String(), secret) {
		t.Fatalf("log output leaked secret: %s", logBuf.String())
	}
}

func TestUnaryInterceptorTolerantOfMissingMetadata(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil)).With("service", "core")

	handler := func(ctx context.Context, req any) (any, error) {
		if got := RequestIDFromContext(ctx); got != "" {
			t.Fatalf("expected empty request id, got %q", got)
		}
		return nil, nil
	}

	if _, err := UnaryRequestIDInterceptor(logger)(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"},
		handler,
	); err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	if !strings.Contains(logBuf.String(), "\"request_id\":\"\"") {
		t.Fatalf("expected empty request id field in logs, got %s", logBuf.String())
	}
}

func TestStreamInterceptorEmitsNormalizedCompletionRecord(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil)).With("service", "document")

	md := metadata.New(map[string]string{"x-request-id": "stream-1"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	stream := &fakeServerStream{ctx: ctx}
	handler := func(srv any, wrapped grpc.ServerStream) error {
		if got := RequestIDFromContext(wrapped.Context()); got != "stream-1" {
			t.Fatalf("stream handler request_id = %q, want %q", got, "stream-1")
		}
		return nil
	}

	if err := StreamRequestIDInterceptor(logger)(
		nil,
		stream,
		&grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"},
		handler,
	); err != nil {
		t.Fatalf("stream interceptor returned error: %v", err)
	}

	entries := decodeLogEntries(t, logBuf.String())
	completed := findEntry(entries, "grpc stream completed")
	if completed == nil {
		t.Fatalf("expected stream completed entry, got %v", entries)
	}
	assertNormalizedFields(t, completed, "document", "stream-1", "/test.Service/Stream", "success", "success")
	assertDurationNumeric(t, completed)
}

func TestStreamInterceptorEmitsNormalizedErrorRecord(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil)).With("service", "document")

	md := metadata.New(map[string]string{"x-request-id": "stream-err"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	stream := &fakeServerStream{ctx: ctx}
	handler := func(srv any, wrapped grpc.ServerStream) error {
		return status.Error(codes.PermissionDenied, "forbidden-detail")
	}

	if err := StreamRequestIDInterceptor(logger)(
		nil,
		stream,
		&grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"},
		handler,
	); err == nil {
		t.Fatal("expected error")
	}

	entries := decodeLogEntries(t, logBuf.String())
	failed := findEntry(entries, "grpc stream failed")
	if failed == nil {
		t.Fatalf("expected stream failed entry, got %v", entries)
	}
	assertNormalizedFields(t, failed, "document", "stream-err", "/test.Service/Stream", "error", "error")
	if class, _ := failed["error_class"].(string); class != "grpc_PERMISSION_DENIED" {
		t.Fatalf("error_class = %q, want grpc_PERMISSION_DENIED", class)
	}
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *fakeServerStream) Context() context.Context { return s.ctx }

func decodeLogEntries(t *testing.T, raw string) []map[string]any {
	t.Helper()
	entries := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid log line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func findEntry(entries []map[string]any, message string) map[string]any {
	for _, entry := range entries {
		if entry["msg"] == message {
			return entry
		}
	}
	return nil
}

func assertNormalizedFields(t *testing.T, entry map[string]any, service, requestID, operation, status, result string) {
	t.Helper()
	if got, _ := entry["service"].(string); got != service {
		t.Fatalf("service = %q, want %q (entry=%v)", got, service, entry)
	}
	if got, _ := entry["request_id"].(string); got != requestID {
		t.Fatalf("request_id = %q, want %q (entry=%v)", got, requestID, entry)
	}
	if got, _ := entry["operation"].(string); got != operation {
		t.Fatalf("operation = %q, want %q (entry=%v)", got, operation, entry)
	}
	if got, _ := entry["status"].(string); got != status {
		t.Fatalf("status = %q, want %q (entry=%v)", got, status, entry)
	}
	if got, _ := entry["result"].(string); got != result {
		t.Fatalf("result = %q, want %q (entry=%v)", got, result, entry)
	}
}

func assertDurationNumeric(t *testing.T, entry map[string]any) {
	t.Helper()
	raw, ok := entry["duration_ms"]
	if !ok {
		t.Fatalf("duration_ms missing from entry %v", entry)
	}
	switch v := raw.(type) {
	case float64:
		if v < 0 {
			t.Fatalf("duration_ms negative = %v", v)
		}
	case int64:
		if v < 0 {
			t.Fatalf("duration_ms negative = %v", v)
		}
	default:
		t.Fatalf("duration_ms has non-numeric type %T", v)
	}
}

func TestUnaryMetricsInterceptorRecordsRequestAndCode(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	handler := func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	}
	if _, err := UnaryMetricsInterceptor(recorder)(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/core.v1.CoreService/Ping"},
		handler,
	); err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	failing := func(_ context.Context, _ any) (any, error) {
		return nil, status.Error(codes.NotFound, "missing")
	}
	if _, err := UnaryMetricsInterceptor(recorder)(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/core.v1.CoreService/Ping"},
		failing,
	); err == nil {
		t.Fatal("expected failing handler error")
	}
	scrape := httptest.NewRecorder()
	recorder.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
	body := scrape.Body.String()
	for _, fragment := range []string{
		`delim_grpc_requests_total{method="/core.v1.CoreService/Ping",result="success",code="ok"}`,
		`delim_grpc_requests_total{method="/core.v1.CoreService/Ping",result="error",code="not_found"}`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("missing metric fragment %q\n%s", fragment, body)
		}
	}
}

func TestUnaryMetricsInterceptorIsNoOpWhenRecorderMissing(t *testing.T) {
	t.Parallel()

	called := false
	handler := func(_ context.Context, _ any) (any, error) {
		called = true
		return nil, nil
	}
	if _, err := UnaryMetricsInterceptor(nil)(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/test/Method"},
		handler,
	); err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	if !called {
		t.Fatal("handler should be invoked when recorder is nil")
	}
}
