package grpcx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

func TestUnaryInterceptorStampsContextAndLogsRequestID(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	md := metadata.New(map[string]string{"x-request-id": "trace-1"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		if got := RequestIDFromContext(ctx); got != "trace-1" {
			t.Fatalf("handler request_id = %q, want %q", got, "trace-1")
		}
		return "ok", nil
	}

	resp, err := UnaryRequestIDInterceptor(logger)(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"},
		handler,
	)
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	if !called {
		t.Fatal("handler was not invoked")
	}
	if resp != "ok" {
		t.Fatalf("response = %v, want %q", resp, "ok")
	}

	entries := decodeLogEntries(t, logBuf.String())
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 log entries, got %d", len(entries))
	}
	if !containsEntry(entries, "grpc request started", "trace-1") {
		t.Fatalf("expected started log to include request id, entries=%v", entries)
	}
	if !containsEntry(entries, "grpc request completed", "trace-1") {
		t.Fatalf("expected completed log to include request id, entries=%v", entries)
	}
}

func TestUnaryInterceptorTolerantOfMissingMetadata(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	var observed string
	handler := func(ctx context.Context, req any) (any, error) {
		observed = RequestIDFromContext(ctx)
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
	if observed != "" {
		t.Fatalf("expected empty request id, got %q", observed)
	}
	if !strings.Contains(logBuf.String(), "\"request_id\":\"\"") {
		t.Fatalf("expected empty request id field in logs, got %s", logBuf.String())
	}
}

func TestStreamInterceptorStampsContextAndLogsRequestID(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	md := metadata.New(map[string]string{"x-request-id": "stream-1"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	stream := &fakeServerStream{ctx: ctx}
	called := false
	handler := func(srv any, wrapped grpc.ServerStream) error {
		called = true
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
	if !called {
		t.Fatal("stream handler was not invoked")
	}

	entries := decodeLogEntries(t, logBuf.String())
	if !containsEntry(entries, "grpc stream started", "stream-1") {
		t.Fatalf("expected started log to include request id, entries=%v", entries)
	}
	if !containsEntry(entries, "grpc stream completed", "stream-1") {
		t.Fatalf("expected completed log to include request id, entries=%v", entries)
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

func containsEntry(entries []map[string]any, message, requestID string) bool {
	for _, entry := range entries {
		if entry["msg"] == message && entry["request_id"] == requestID {
			return true
		}
	}
	return false
}