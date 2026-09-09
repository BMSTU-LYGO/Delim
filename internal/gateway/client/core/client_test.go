package core

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"delim/pkg/metricsx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDeadlineUnaryInterceptorAddsDeadline(t *testing.T) {
	t.Parallel()

	var deadline time.Time
	err := deadlineUnaryInterceptor(
		context.Background(),
		"/test.Service/Method",
		nil,
		nil,
		nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			var ok bool
			deadline, ok = ctx.Deadline()
			if !ok {
				t.Fatal("downstream call has no deadline")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("interceptor returned an error: %v", err)
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > defaultTimeout {
		t.Fatalf("unexpected downstream deadline: %s", remaining)
	}
}

func TestDeadlineUnaryInterceptorKeepsCallerDeadline(t *testing.T) {
	t.Parallel()

	want := time.Now().Add(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), want)
	defer cancel()

	err := deadlineUnaryInterceptor(
		ctx,
		"/test.Service/Method",
		nil,
		nil,
		nil,
		func(callCtx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			got, ok := callCtx.Deadline()
			if !ok || !got.Equal(want) {
				t.Fatalf("deadline = %v, want %v", got, want)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("interceptor returned an error: %v", err)
	}
}

func TestClientMetricsInterceptorRecordsErrorWithBoundedPeer(t *testing.T) {
	t.Parallel()

	recorder := metricsx.NoOp()
	interceptor := clientMetricsInterceptor(recorder)
	err := interceptor(
		context.Background(),
		"/core.v1.CoreService/Ping",
		nil,
		nil,
		nil,
		func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			return status.Error(codes.NotFound, "missing")
		},
	)
	if err == nil {
		t.Fatal("expected error from invoker")
	}
	scrape := httptest.NewRecorder()
	recorder.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
	body := scrape.Body.String()
	if !strings.Contains(body, `delim_grpc_client_errors_total{peer="core",operation="/core.v1.CoreService/Ping",result="error"}`) {
		t.Fatalf("client error counter missing: %s", body)
	}
}
