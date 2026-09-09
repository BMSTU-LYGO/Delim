package grpcx

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

// UnaryRequestIDInterceptor returns a gRPC server unary interceptor that
// extracts the request id from incoming metadata, stores it on the context
// for handlers, and logs each request through the supplied slog.Logger.
func UnaryRequestIDInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := extractRequestID(ctx)
		ctx = context.WithValue(ctx, requestIDContextKey{}, id)
		started := time.Now()
		logger := log.With("request_id", id, "grpc.method", info.FullMethod)
		logger.Info("grpc request started")
		response, err := handler(ctx, req)
		if err != nil {
			logger.Error("grpc request failed", "error", err.Error(), "duration", time.Since(started))
			return nil, err
		}
		logger.Info("grpc request completed", "duration", time.Since(started))
		return response, nil
	}
}

// StreamRequestIDInterceptor returns a gRPC server stream interceptor that
// extracts the request id from incoming metadata, wraps the stream with a
// context that exposes it, and logs each stream through the supplied logger.
func StreamRequestIDInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		id := extractRequestID(stream.Context())
		ctx := context.WithValue(stream.Context(), requestIDContextKey{}, id)
		wrapped := &requestIDServerStream{ServerStream: stream, ctx: ctx}
		started := time.Now()
		logger := log.With("request_id", id, "grpc.method", info.FullMethod)
		logger.Info("grpc stream started")
		err := handler(srv, wrapped)
		if err != nil {
			logger.Error("grpc stream failed", "error", err.Error(), "duration", time.Since(started))
			return err
		}
		logger.Info("grpc stream completed", "duration", time.Since(started))
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

type requestIDServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *requestIDServerStream) Context() context.Context {
	return s.ctx
}