package http

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"delim/pkg/metricsx"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDPropagatesToGRPCMetadata(t *testing.T) {
	t.Parallel()

	const requestIDValue = "request-from-client"
	var propagated string
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, _ := metadata.FromOutgoingContext(r.Context())
		propagated = values.Get("x-request-id")[0]
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", requestIDValue)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if propagated != requestIDValue {
		t.Fatalf("propagated request id = %q, want %q", propagated, requestIDValue)
	}
	if got := response.Header().Get("X-Request-ID"); got != requestIDValue {
		t.Fatalf("response request id = %q, want %q", got, requestIDValue)
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	t.Parallel()

	var propagated string
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, _ := metadata.FromOutgoingContext(r.Context())
		propagated = values.Get("x-request-id")[0]
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if propagated == "" {
		t.Fatal("expected generated request id in outgoing metadata, got empty")
	}
	if got := response.Header().Get("X-Request-ID"); got != propagated {
		t.Fatalf("response request id = %q, want %q", got, propagated)
	}
	if !isValidRequestID(propagated) {
		t.Fatalf("generated id %q is not valid", propagated)
	}
}

func TestRequestIDRejectsControlCharactersAndRegenerates(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"newline":      "abc\ndef",
		"carriage":     "abc\rdef",
		"tab":          "abc\tdef",
		"space":        "abc def",
		"semicolon":    "abc;def",
		"header_break": "abc\r\nInjected: header",
		"non_ascii":    "abc-идентификатор",
		"too_long":     strings.Repeat("a", maxRequestIDLength+1),
	}

	for name, raw := range cases {
		raw := raw
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var propagated string
			handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				values, _ := metadata.FromOutgoingContext(r.Context())
				propagated = values.Get("x-request-id")[0]
				w.WriteHeader(http.StatusNoContent)
			}))

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("X-Request-ID", raw)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if propagated == "" {
				t.Fatal("expected regenerated request id, got empty")
			}
			if propagated == raw {
				t.Fatalf("invalid id %q was propagated unchanged", raw)
			}
			if !isValidRequestID(propagated) {
				t.Fatalf("regenerated id %q is not valid", propagated)
			}
			if got := response.Header().Get("X-Request-ID"); got != propagated {
				t.Fatalf("response request id = %q, want %q", got, propagated)
			}
		})
	}
}

func TestRequestIDAcceptsBoundaryLength(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("a", maxRequestIDLength)
	var propagated string
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, _ := metadata.FromOutgoingContext(r.Context())
		propagated = values.Get("x-request-id")[0]
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", raw)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if propagated != raw {
		t.Fatalf("boundary length id not preserved: got %q, want %q", propagated, raw)
	}
}

func TestGenerateRequestIDWithSuccessPathReturnsHex(t *testing.T) {
	t.Parallel()

	reader := func(dst []byte) error {
		for i := range dst {
			dst[i] = byte(i)
		}
		return nil
	}
	clock := func() time.Time { return time.Unix(0, 0).UTC() }

	id := generateRequestIDWith(reader, clock)
	want := hex.EncodeToString([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	if id != want {
		t.Fatalf("crypto-path id = %q, want %q", id, want)
	}
	if !isValidRequestID(id) {
		t.Fatalf("crypto-path id %q is not valid", id)
	}
	if len(id) != 32 {
		t.Fatalf("crypto-path id length = %d, want 32", len(id))
	}
}

func TestGenerateRequestIDWithFailingReaderReturnsBoundedSafeFallback(t *testing.T) {
	t.Parallel()

	failingReader := func([]byte) error { return errors.New("simulated entropy failure") }
	clock := func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

	id := generateRequestIDWith(failingReader, clock)

	if id == "" {
		t.Fatal("expected nonempty fallback id")
	}
	if !isValidRequestID(id) {
		t.Fatalf("fallback id %q is not valid (must match header-safe subset)", id)
	}
	if len(id) > maxRequestIDLength {
		t.Fatalf("fallback id length %d exceeds bound %d", len(id), maxRequestIDLength)
	}
	if !strings.HasPrefix(id, "fb-") {
		t.Fatalf("fallback id %q must be prefixed with fb- to mark non-cryptographic origin", id)
	}

	second := generateRequestIDWith(failingReader, clock)
	if second == "" {
		t.Fatal("expected nonempty fallback id on second call")
	}
	if second == id {
		t.Fatalf("fallback ids must be distinct across calls, got %q twice", id)
	}
}

func TestFallbackRequestIDRespectsFixedClock(t *testing.T) {
	t.Parallel()

	clock := func() time.Time { return time.Unix(1700000000, 0).UTC() }

	id := fallbackRequestID(clock)
	if id == "" {
		t.Fatal("expected nonempty fallback id")
	}
	if !strings.HasPrefix(id, "fb-") {
		t.Fatalf("fallback %q missing fb- prefix", id)
	}
	if !isValidRequestID(id) {
		t.Fatalf("fallback %q must be header-safe", id)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 3 || parts[0] != "fb" {
		t.Fatalf("fallback %q must follow fb-<hex>-<hex> shape", id)
	}
	if len(parts[1]) != 16 || len(parts[2]) != 16 {
		t.Fatalf("fallback %q must have 16-char hex segments", id)
	}
}

func TestResponseWriterPreservesFlushing(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	writer := &responseWriter{ResponseWriter: response, status: http.StatusOK}
	writer.Flush()
	if !response.Flushed {
		t.Fatal("wrapped response writer did not flush")
	}
}

func TestCORSAllowsConfiguredFrontendOriginAndMethods(t *testing.T) {
	t.Parallel()

	handler := cors([]string{"https://miniapp.example"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/me", nil)
	request.Header.Set("Origin", "https://miniapp.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://miniapp.example" {
		t.Fatalf("allowed origin = %q, want configured origin", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
		t.Fatalf("allowed methods = %q", got)
	}
}

func TestCORSDoesNotTrustUnknownOrigin(t *testing.T) {
	t.Parallel()

	handler := cors([]string{"https://miniapp.example"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/me", nil)
	request.Header.Set("Origin", "https://unknown.example")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allowed unknown origin = %q", got)
	}
}

func TestAccessLogEmitsNormalizedFields(t *testing.T) {
	t.Parallel()

	logBuf, logger := captureJSONLogger("gateway")
	handler := requestID(accessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("X-Request-ID", "trace-1")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	entry := findLogEntry(t, logBuf, "http request")
	if got, _ := entry["service"].(string); got != "gateway" {
		t.Fatalf("service = %q, want gateway", got)
	}
	if got, _ := entry["request_id"].(string); got != "trace-1" {
		t.Fatalf("request_id = %q, want trace-1", got)
	}
	if got, _ := entry["operation"].(string); got != "GET /api/v1/me" {
		t.Fatalf("operation = %q, want \"GET /api/v1/me\"", got)
	}
	assertNumericDuration(t, entry)
	if got, ok := entry["status"].(float64); !ok || got != float64(http.StatusOK) {
		t.Fatalf("status = %v, want 200", entry["status"])
	}
	if got, _ := entry["result"].(string); got != "success" {
		t.Fatalf("result = %q, want success", got)
	}
	if _, ok := entry["error_class"]; ok {
		t.Fatalf("success record must not include error_class: %v", entry)
	}
}

func TestMetricsMiddlewareRecordsBoundedRouteAndStatus(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	router := chi.NewRouter()
	router.Use(metricsMiddleware(recorder))
	router.Get("/api/v1/groups/{groupID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Post("/api/v1/max/webhook", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	for _, path := range []string{"/api/v1/groups/123", "/api/v1/max/webhook"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(httptest.NewRecorder(), req)
	}

	handler := recorder.Handler()
	scrape := httptest.NewRecorder()
	handler.ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := scrape.Body.String()
	if !strings.Contains(body, `delim_http_requests_total{method="GET",route="/api/v1/groups/{groupID}",status_class="2xx",result="success"}`) {
		t.Fatalf("missing 2xx counter: %s", body)
	}
	if strings.Contains(body, `route="/api/v1/groups/123"`) {
		t.Fatalf("route label leaked raw URL id: %s", body)
	}
}

func TestAccessLogEmitsErrorResultOn4xx(t *testing.T) {
	t.Parallel()

	logBuf, logger := captureJSONLogger("gateway")
	handler := requestID(accessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	entry := findLogEntry(t, logBuf, "http request")
	if got, _ := entry["result"].(string); got != "error" {
		t.Fatalf("result = %q, want error", got)
	}
}

func TestAccessLogEmitsErrorResultOn5xx(t *testing.T) {
	t.Parallel()

	logBuf, logger := captureJSONLogger("gateway")
	handler := requestID(accessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/groups", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	entry := findLogEntry(t, logBuf, "http request")
	if got, _ := entry["result"].(string); got != "error" {
		t.Fatalf("result = %q, want error", got)
	}
	if got, ok := entry["status"].(float64); !ok || got != float64(http.StatusInternalServerError) {
		t.Fatalf("status = %v, want 500", entry["status"])
	}
}

func TestAccessLogDoesNotLogAuthorizationOrInitData(t *testing.T) {
	t.Parallel()

	logBuf, logger := captureJSONLogger("gateway")
	handler := requestID(accessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	authToken := "Bearer supersecret-session-token"
	initData := "raw-init-data-with-secret"
	botToken := "MAX_BOT_TOKEN=abc123"
	request.Header.Set("Authorization", authToken)
	request.Header.Set("X-Max-Bot-Api-Secret", botToken)
	request.Header.Set("X-Max-Init-Data", initData)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	output := logBuf.String()
	for _, sensitive := range []string{authToken, initData, botToken, "supersecret-session-token", "raw-init-data-with-secret"} {
		if strings.Contains(output, sensitive) {
			t.Fatalf("access log leaked sensitive value %q: %s", sensitive, output)
		}
	}
}

func TestRecovererEmitsNormalizedErrorRecordAndGenericResponse(t *testing.T) {
	t.Parallel()

	logBuf, logger := captureJSONLogger("gateway")
	handler := requestID(recoverer(logger)(accessLog(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("super-secret-payload-must-not-leak")
	}))))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("X-Request-ID", "panic-trace")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"code":"internal_error"`) {
		t.Fatalf("response body missing internal_error code: %s", body)
	}
	if strings.Contains(body, "super-secret-payload") {
		t.Fatalf("response body leaked panic value: %s", body)
	}
	if strings.Contains(body, "goroutine") || strings.Contains(body, "panic") {
		t.Fatalf("response body leaked stack/panic detail: %s", body)
	}

	entry := findLogEntry(t, logBuf, "http panic")
	if got, _ := entry["service"].(string); got != "gateway" {
		t.Fatalf("service = %q, want gateway", got)
	}
	if got, _ := entry["request_id"].(string); got != "panic-trace" {
		t.Fatalf("request_id = %q, want panic-trace", got)
	}
	if got, _ := entry["operation"].(string); got != "GET /api/v1/me" {
		t.Fatalf("operation = %q, want \"GET /api/v1/me\"", got)
	}
	if got, _ := entry["error_class"].(string); got != "panic" {
		t.Fatalf("error_class = %q, want panic", got)
	}
	if got, _ := entry["result"].(string); got != "error" {
		t.Fatalf("result = %q, want error", got)
	}
}

func TestHTTPResultClassifiesStatus(t *testing.T) {
	t.Parallel()

	if got := httpResult(http.StatusOK); got != "success" {
		t.Fatalf("httpResult(200) = %q, want success", got)
	}
	if got := httpResult(http.StatusNoContent); got != "success" {
		t.Fatalf("httpResult(204) = %q, want success", got)
	}
	if got := httpResult(http.StatusBadRequest); got != "error" {
		t.Fatalf("httpResult(400) = %q, want error", got)
	}
	if got := httpResult(http.StatusUnauthorized); got != "error" {
		t.Fatalf("httpResult(401) = %q, want error", got)
	}
	if got := httpResult(http.StatusInternalServerError); got != "error" {
		t.Fatalf("httpResult(500) = %q, want error", got)
	}
}

func captureJSONLogger(service string) (*bytes.Buffer, *slog.Logger) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil)).With("service", service)
	return &buf, logger
}

func findLogEntry(t *testing.T, buf *bytes.Buffer, message string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid log line %q: %v", line, err)
		}
		if entry["msg"] == message {
			return entry
		}
	}
	t.Fatalf("no log entry with msg=%q in %s", message, buf.String())
	return nil
}

func assertNumericDuration(t *testing.T, entry map[string]any) {
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
