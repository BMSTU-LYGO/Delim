package grpcx

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewServer(options ...grpc.ServerOption) *grpc.Server {
	return grpc.NewServer(options...)
}

func NewClient(target string, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	baseOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	conn, err := grpc.NewClient(target, append(baseOptions, options...)...)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", target, err)
	}

	return conn, nil
}
