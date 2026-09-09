package http

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"delim/internal/gateway/ratelimit"
)

func TestRateLimitMiddlewareRejectsOverBurstWithRetryAfter(t *testing.T) {
	t.Parallel()

	limiter := ratelimit.New(2, 100) // burst of 2
	handler := rateLimit(limiter, clientIPKey)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/max", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, response.Code)
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/max", nil))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", response.Code)
	}
	if retry := response.Header().Get("Retry-After"); retry == "" {
		t.Fatal("missing Retry-After header")
	} else if seconds, err := strconv.Atoi(retry); err != nil || seconds < 1 {
		t.Fatalf("invalid Retry-After %q", retry)
	}
}

func TestRateLimitMiddlewareNilLimiterIsPassThrough(t *testing.T) {
	t.Parallel()

	handler := rateLimit(nil, clientIPKey)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for i := 0; i < 1000; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, response.Code)
		}
	}
}

func TestClientIPKeyUsesTransportAddressOnly(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.7:54321"
	request.Header.Set("X-Forwarded-For", "10.0.0.1")
	if got := clientIPKey(request); got != "203.0.113.7" {
		t.Fatalf("clientIPKey = %q, want transport IP only", got)
	}
}
