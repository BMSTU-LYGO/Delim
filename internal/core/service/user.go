package service

import (
	"context"

	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
)

type UserService interface {
	Upsert(context.Context, domain.User) (domain.User, error)
	Get(context.Context, int64) (domain.User, error)
}

func (s *GRPCServer) UpsertUser(ctx context.Context, req *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error) {
	user, err := s.users.Upsert(ctx, domain.User{MaxUserID: req.GetMaxUserId(), FirstName: req.GetFirstName(), LastName: req.GetLastName(), Username: req.GetUsername()})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.UpsertUserResponse{User: userToProto(user)}, nil
}

func (s *GRPCServer) GetUser(ctx context.Context, req *corev1.GetUserRequest) (*corev1.GetUserResponse, error) {
	user, err := s.users.Get(ctx, req.GetId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.GetUserResponse{User: userToProto(user)}, nil
}

func userToProto(user domain.User) *corev1.User {
	return &corev1.User{Id: user.ID, MaxUserId: user.MaxUserID, FirstName: user.FirstName, LastName: user.LastName, Username: user.Username, CreatedAtUnix: user.CreatedAt.Unix(), UpdatedAtUnix: user.UpdatedAt.Unix()}
}
