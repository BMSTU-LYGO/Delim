package http

import (
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
