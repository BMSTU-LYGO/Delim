package metricsx

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func grpcCodeFromError(err error) string {
	if err == nil {
		return "ok"
	}
	if s, ok := status.FromError(err); ok {
		return normalizeCode(s.Code())
	}
	if err == context.Canceled {
		return "canceled"
	}
	if err == context.DeadlineExceeded {
		return "deadline_exceeded"
	}
	return "internal"
}

func normalizeCode(code codes.Code) string {
	switch code {
	case codes.OK:
		return "ok"
	case codes.Canceled:
		return "canceled"
	case codes.Unknown:
		return "unknown"
	case codes.InvalidArgument:
		return "invalid_argument"
	case codes.DeadlineExceeded:
		return "deadline_exceeded"
	case codes.NotFound:
		return "not_found"
	case codes.AlreadyExists:
		return "already_exists"
	case codes.PermissionDenied:
		return "permission_denied"
	case codes.ResourceExhausted:
		return "resource_exhausted"
	case codes.FailedPrecondition:
		return "failed_precondition"
	case codes.Aborted:
		return "aborted"
	case codes.OutOfRange:
		return "out_of_range"
	case codes.Unimplemented:
		return "unimplemented"
	case codes.Internal:
		return "internal"
	case codes.Unavailable:
		return "unavailable"
	case codes.DataLoss:
		return "data_loss"
	case codes.Unauthenticated:
		return "unauthenticated"
	default:
		return "unknown"
	}
}
