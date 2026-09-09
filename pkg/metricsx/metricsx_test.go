package metricsx_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"delim/pkg/metricsx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestObserveHTTPEmitsBoundedLabels(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveHTTP("GET", "/api/v1/groups/{groupID}", http.StatusOK, 12*time.Millisecond)
	recorder.ObserveHTTP("GET", "/api/v1/groups/{groupID}", http.StatusNotFound, 3*time.Millisecond)
	recorder.ObserveHTTP("POST", "/api/v1/max/webhook", http.StatusUnauthorized, 6*time.Millisecond)

	body := scrape(t, recorder.Handler())
	mustContain(t, body, `delim_http_requests_total{method="GET",route="/api/v1/groups/{groupID}",status_class="2xx",result="success"}`)
	mustContain(t, body, `delim_http_requests_total{method="GET",route="/api/v1/groups/{groupID}",status_class="4xx",result="error"}`)
	mustContain(t, body, `delim_http_requests_total{method="POST",route="/api/v1/max/webhook",status_class="4xx",result="error"}`)
	mustContain(t, body, `delim_http_request_duration_seconds_count{method="GET",route="/api/v1/groups/{groupID}",status_class="2xx"}`)
	mustContain(t, body, `delim_http_request_duration_seconds_sum`)
}

func TestObserveGRPCServerEmitsBoundedCode(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveGRPCServer("/core.v1.CoreService/CreateExpense", nil, 5*time.Millisecond)
	recorder.ObserveGRPCServer("/core.v1.CoreService/CreateExpense", status.Error(codes.InvalidArgument, "bad"), 4*time.Millisecond)
	recorder.ObserveGRPCServer("/core.v1.CoreService/UpdateExpense", status.Error(codes.FailedPrecondition, "stale"), 6*time.Millisecond)

	body := scrape(t, recorder.Handler())
	mustContain(t, body, `delim_grpc_requests_total{method="/core.v1.CoreService/CreateExpense",result="success",code="ok"}`)
	mustContain(t, body, `delim_grpc_requests_total{method="/core.v1.CoreService/CreateExpense",result="error",code="invalid_argument"}`)
	mustContain(t, body, `delim_grpc_requests_total{method="/core.v1.CoreService/UpdateExpense",result="error",code="failed_precondition"}`)
}

func TestObserveWebhookOnlyAllowsBoundedResults(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveWebhook("accepted")
	recorder.ObserveWebhook("failed")
	recorder.ObserveWebhook("user-controlled-data")
	recorder.ObserveWebhook("ignored")

	body := scrape(t, recorder.Handler())
	mustContain(t, body, `delim_webhook_events_total{result="accepted"}`)
	mustContain(t, body, `delim_webhook_events_total{result="failed"}`)
	mustContain(t, body, `delim_webhook_events_total{result="ignored"}`)
	if strings.Contains(body, `result="user-controlled-data"`) {
		t.Fatalf("webhook result label accepted user-controlled value: %s", body)
	}
}

func TestObserveMAXAPIErrorNormalisesStatusClass(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveMAXAPIError("send_message", "5xx", errors.New("boom"))
	recorder.ObserveMAXAPIError("send_message", "transport", errors.New("connection refused"))

	body := scrape(t, recorder.Handler())
	mustContain(t, body, `delim_max_api_errors_total{operation="send_message",status_class="5xx",result="error"}`)
	mustContain(t, body, `delim_max_api_errors_total{operation="send_message",status_class="transport",result="error"}`)
}

func TestObserveOCRJobRejectsUnknownStates(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveOCRJob("completed")
	recorder.ObserveOCRJob("unknown-state")
	recorder.ObserveOCRJob("failed")

	body := scrape(t, recorder.Handler())
	mustContain(t, body, `delim_ocr_jobs_total{state="completed"}`)
	mustContain(t, body, `delim_ocr_jobs_total{state="failed"}`)
	if strings.Contains(body, `state="unknown-state"`) {
		t.Fatalf("ocr state label accepted unbounded value: %s", body)
	}
}

func TestObserveExportJobNormalisesFormat(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveExportJob("PDF", "success")
	recorder.ObserveExportJob("XLSX", "error")
	recorder.ObserveExportJob("docx", "success")

	body := scrape(t, recorder.Handler())
	mustContain(t, body, `delim_export_jobs_total{format="pdf",result="success"}`)
	mustContain(t, body, `delim_export_jobs_total{format="xlsx",result="error"}`)
	if strings.Contains(body, `format="docx"`) {
		t.Fatalf("export format label accepted unbounded value: %s", body)
	}
}

func TestObserveMinIOErrorUsesBoundedOperation(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveMinIOError("put", nil)
	recorder.ObserveMinIOError("delete", errors.New("nope"))
	recorder.ObserveMinIOError("Put", nil)

	body := scrape(t, recorder.Handler())
	mustContain(t, body, `delim_minio_errors_total{operation="put",result="success"}`)
	mustContain(t, body, `delim_minio_errors_total{operation="delete",result="error"}`)
	if strings.Contains(body, `operation="rm"`) {
		t.Fatalf("minio operation label accepted unbounded value: %s", body)
	}
}

func TestObserveConflictRecordsBusinessConflict(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveConflict("UpdateExpense")
	recorder.ObserveConflict("UpdateExpense")
	recorder.ObserveConflict("CreateAdjustment")

	body := scrape(t, recorder.Handler())
	mustContain(t, body, `delim_conflicts_total{operation="UpdateExpense"}`)
	mustContain(t, body, `delim_conflicts_total{operation="CreateAdjustment"}`)
}

func TestNewFailsLoudWhenPortBusy(t *testing.T) {
	t.Parallel()

	addr, err := reserveLoopback()
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer addr.Close()

	host, port, err := splitHostPort(addr.Addr().String())
	if err != nil {
		t.Fatalf("parse address: %v", err)
	}

	_, _, err = metricsx.New(metricsx.Config{Host: host, Port: port}, metricsx.Options{Service: "gateway"})
	if err == nil {
		t.Fatal("expected metricsx.New to fail when port is already bound")
	}
}

func TestNewShutdownIsIdempotent(t *testing.T) {
	t.Parallel()

	addr, err := reserveLoopback()
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer addr.Close()
	_ = addr.Close()

	host, port, err := splitHostPort(addr.Addr().String())
	if err != nil {
		t.Fatalf("parse address: %v", err)
	}

	recorder, shutdown, err := metricsx.New(metricsx.Config{Host: host, Port: port}, metricsx.Options{Service: "gateway"})
	if err != nil {
		t.Fatalf("metricsx.New: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
	if err := recorder.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown method: %v", err)
	}
}

func TestRecorderIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recorder.ObserveHTTP("GET", "/api/v1/me", http.StatusOK, time.Millisecond)
			recorder.ObserveGRPCServer("/core.v1.CoreService/Ping", nil, time.Millisecond)
		}()
	}
	wg.Wait()
	if recorder.Service() != "noop" {
		t.Fatalf("unexpected service label: %q", recorder.Service())
	}
}

func TestWriteTextUsesPrometheusExpositionFormat(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	recorder.ObserveHTTP("GET", "/api/v1/me", http.StatusOK, 5*time.Millisecond)
	var buf bytes.Buffer
	recorder.WriteText(&buf)
	body := buf.String()
	if !strings.Contains(body, "# HELP delim_http_requests_total") {
		t.Fatalf("missing HELP comment: %s", body)
	}
	if !strings.Contains(body, "# TYPE delim_http_requests_total counter") {
		t.Fatalf("missing TYPE comment: %s", body)
	}
}

func TestHTTPStatusClassBoundedValues(t *testing.T) {
	t.Parallel()

	cases := map[int]string{
		100: "1xx", 199: "1xx",
		200: "2xx", 299: "2xx",
		300: "3xx", 399: "3xx",
		400: "4xx", 499: "4xx",
		500: "5xx", 599: "5xx",
	}
	for status, want := range cases {
		if got := metricsx.HTTPStatusClass(status); got != want {
			t.Fatalf("HTTPStatusClass(%d)=%q, want %q", status, got, want)
		}
	}
}

func scrape(t *testing.T, handler http.Handler) string {
	t.Helper()
	if handler == nil {
		t.Fatal("handler is nil")
	}
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("metrics scrape status = %d", recorder.Code)
	}
	return recorder.Body.String()
}

func mustContain(t *testing.T, body, fragment string) {
	t.Helper()
	if !strings.Contains(body, fragment) {
		t.Fatalf("metrics output missing fragment %q\n---\n%s\n---", fragment, body)
	}
}

func reserveLoopback() (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	return listener, nil
}

func splitHostPort(value string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, err
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return "", 0, err
	}
	return host, port, nil
}
