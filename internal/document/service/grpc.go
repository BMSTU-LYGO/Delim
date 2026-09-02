package service

import (
	"context"

	documentv1 "delim/pkg/gen/document/v1"
)

type GRPCServer struct {
	documentv1.UnimplementedDocumentServiceServer
}

func NewGRPCServer() *GRPCServer {
	return &GRPCServer{}
}

func (s *GRPCServer) Ping(context.Context, *documentv1.PingRequest) (*documentv1.PingResponse, error) {
	return &documentv1.PingResponse{Status: "ok"}, nil
}
