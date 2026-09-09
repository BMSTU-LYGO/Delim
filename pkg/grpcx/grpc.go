package grpcx

import (
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewServer(log *slog.Logger, options ...grpc.ServerOption) *grpc.Server {
	baseOptions := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(UnaryRequestIDInterceptor(log)),
		grpc.ChainStreamInterceptor(StreamRequestIDInterceptor(log)),
	}
	return grpc.NewServer(append(baseOptions, options...)...)
}

func NewClient(target string, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	baseOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	conn, err := grpc.NewClient(target, append(baseOptions, options...)...)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", target, err)
	}

	return conn, nil
}
