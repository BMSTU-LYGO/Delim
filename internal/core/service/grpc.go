package service

import (
	"context"

	corev1 "delim/pkg/gen/core/v1"
)

type GRPCServer struct {
	corev1.UnimplementedCoreServiceServer
	users    UserService
	groups   GroupService
	expenses ExpenseService
}

func NewGRPCServer(users UserService, groups GroupService, expenses ExpenseService) *GRPCServer {
	return &GRPCServer{users: users, groups: groups, expenses: expenses}
}

func (s *GRPCServer) Ping(context.Context, *corev1.PingRequest) (*corev1.PingResponse, error) {
	return &corev1.PingResponse{Status: "ok"}, nil
}
