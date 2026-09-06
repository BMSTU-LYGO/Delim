package core

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
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
