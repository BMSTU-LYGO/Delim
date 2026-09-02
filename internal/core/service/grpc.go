package service

import (
	"context"

	corev1 "delim/pkg/gen/core/v1"
)

type GRPCServer struct {
	corev1.UnimplementedCoreServiceServer
}

func NewGRPCServer() *GRPCServer {
	return &GRPCServer{}
}

func (s *GRPCServer) Ping(context.Context, *corev1.PingRequest) (*corev1.PingResponse, error) {
	return &corev1.PingResponse{Status: "ok"}, nil
}
