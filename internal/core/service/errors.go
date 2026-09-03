package service

import (
	"delim/internal/core/domain"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, "invalid argument")
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, "forbidden")
	case errors.Is(err, domain.ErrConflict):
		return status.Error(codes.Aborted, "conflict")
	case errors.Is(err, domain.ErrInvalidState):
		return status.Error(codes.FailedPrecondition, "invalid state")
	case errors.Is(err, domain.ErrArchivedGroup):
		return status.Error(codes.FailedPrecondition, "group is archived")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
