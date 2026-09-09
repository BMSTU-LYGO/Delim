package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakeHealth struct{ err error }

func (f fakeHealth) Ping(context.Context) error { return f.err }

type fakeDocument struct {
	pingErr error
	ocr     string
	ocrErr  error
}

func (f fakeDocument) Ping(context.Context) error { return f.pingErr }

func (f fakeDocument) OCRStatus(context.Context) (string, error) { return f.ocr, f.ocrErr }

func readReady(t *testing.T, handler http.Handler) (map[string]any, int) {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode readiness body %q: %v", rec.Body.String(), err)
	}
	return body, rec.Code
}

func TestReadinessReportsDegradedOCRWithoutFailing(t *testing.T) {
	t.Parallel()
	router := newTestReadinessRouter(
		fakeHealth{}, fakeDocument{ocr: "degraded"}, fakeHealth{},
	)
	body, code := readReady(t, router)
	if code != http.StatusOK {
		t.Fatalf("status code = %d, want 200 (degraded OCR must not fail readiness)", code)
	}
	if body["status"] != "ok" || body["document"] != "ok" {
		t.Fatalf("expected ready+document ok, got %v", body)
	}
	if body["ocr"] != "degraded" {
		t.Fatalf("ocr = %v, want degraded", body["ocr"])
	}
}

func TestReadinessOKWhenOCROk(t *testing.T) {
	t.Parallel()
	router := newTestReadinessRouter(
		fakeHealth{}, fakeDocument{ocr: "ok"}, fakeHealth{},
	)
	body, code := readReady(t, router)
	if code != http.StatusOK || body["ocr"] != "ok" {
		t.Fatalf("expected 200 with ocr=ok, got code=%d body=%v", code, body)
	}
}

func newTestReadinessRouter(core healthChecker, document documentReadiness, postgres healthChecker) http.Handler {
	router := chi.NewRouter()
	router.Get("/health/ready", readiness(core, document, postgres))
	return router
}
