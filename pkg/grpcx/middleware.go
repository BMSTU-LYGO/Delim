package grpcx

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"delim/pkg/metricsx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const RequestIDMetadataKey = "x-request-id"

type requestIDContextKey struct{}

// RequestIDFromContext returns the request id stored on the context by the
// unary/streaming interceptors. It is safe to call on any context.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return id
	}
	return ""
}

// ErrorClass returns a bounded string identifying the error class for log
// records. It never includes the full error message or stack trace and is
// safe to emit at any log level.
func ErrorClass(err error) string {
	if err == nil {
		return "ok"
	}
	if s, ok := status.FromError(err); ok {
		return "grpc_" + screamingSnake(s.Code().String())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "internal"
}

func screamingSnake(value string) string {
	result := make([]byte, 0, len(value)+4)
	for index := 0; index < len(value); index++ {
		current := value[index]
		if current >= 'A' && current <= 'Z' {
			if index > 0 {
				result = append(result, '_')
			}
			result = append(result, current)
			continue
		}
		if current >= 'a' && current <= 'z' {
			result = append(result, current-'a'+'A')
			continue
		}
		result = append(result, current)
	}
	return string(result)
}

// UnaryRequestIDInterceptor returns a gRPC server unary interceptor that
// extracts the request id from incoming metadata, stores it on the context
// for handlers, and emits a normalized structured completion record.
func UnaryRequestIDInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := extractRequestID(ctx)
		ctx = context.WithValue(ctx, requestIDContextKey{}, id)
		started := time.Now()
		logger := log.With(
			"request_id", id,
			"operation", info.FullMethod,
		)
		response, err := handler(ctx, req)
		durationMs := time.Since(started).Milliseconds()
		if err != nil {
			logger.Error("grpc request failed",
				"duration_ms", durationMs,
				"status", "error",
				"result", "error",
				"error_class", ErrorClass(err),
			)
			return nil, err
		}
		logger.Info("grpc request completed",
			"duration_ms", durationMs,
			"status", "success",
			"result", "success",
		)
		return response, nil
	}
}

// StreamRequestIDInterceptor returns a gRPC server stream interceptor that
// extracts the request id from incoming metadata, wraps the stream with a
// context that exposes it, and emits a normalized structured completion
// record.
func StreamRequestIDInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		id := extractRequestID(stream.Context())
		ctx := context.WithValue(stream.Context(), requestIDContextKey{}, id)
		wrapped := &requestIDServerStream{ServerStream: stream, ctx: ctx}
		started := time.Now()
		logger := log.With(
			"request_id", id,
			"operation", info.FullMethod,
		)
		err := handler(srv, wrapped)
		durationMs := time.Since(started).Milliseconds()
		if err != nil {
			logger.Error("grpc stream failed",
				"duration_ms", durationMs,
				"status", "error",
				"result", "error",
				"error_class", ErrorClass(err),
			)
			return err
		}
		logger.Info("grpc stream completed",
			"duration_ms", durationMs,
			"status", "success",
			"result", "success",
		)
		return nil
	}
}

func extractRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(RequestIDMetadataKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// UnaryMetricsInterceptor returns a server interceptor that records request
// count/duration metrics and never blocks when recorder is nil.
func UnaryMetricsInterceptor(recorder *metricsx.Recorder) grpc.UnaryServerInterceptor {
	if recorder == nil {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		resp, err := handler(ctx, req)
		recorder.ObserveGRPCServer(info.FullMethod, err, time.Since(started))
		return resp, err
	}
}

// StreamMetricsInterceptor returns a server stream interceptor that records
// request count/duration metrics for completed streams.
func StreamMetricsInterceptor(recorder *metricsx.Recorder) grpc.StreamServerInterceptor {
	if recorder == nil {
		return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, stream)
		}
	}
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		started := time.Now()
		err := handler(srv, stream)
		recorder.ObserveGRPCServer(info.FullMethod, err, time.Since(started))
		return err
	}
}

type requestIDServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *requestIDServerStream) Context() context.Context {
	return s.ctx
}
