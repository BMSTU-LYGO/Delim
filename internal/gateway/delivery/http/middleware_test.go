package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestResponseWriterPreservesFlushing(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	writer := &responseWriter{ResponseWriter: response, status: http.StatusOK}
	writer.Flush()
	if !response.Flushed {
		t.Fatal("wrapped response writer did not flush")
	}
}
