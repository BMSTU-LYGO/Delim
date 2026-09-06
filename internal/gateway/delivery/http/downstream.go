package http

import (
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func writeDownstreamError(w http.ResponseWriter, err error) {
	httpStatus := http.StatusInternalServerError
	code := "internal_error"
	message := "internal server error"

	grpcStatus := status.Convert(err)
	tooLarge := isTooLargeGRPCError(grpcStatus.Message())
	switch grpcStatus.Code() {
	case codes.InvalidArgument:
		if tooLarge {
			httpStatus, code, message = http.StatusRequestEntityTooLarge, "payload_too_large", "request payload is too large"
		} else {
			httpStatus, code, message = http.StatusBadRequest, "invalid_argument", "invalid request"
		}
	case codes.Unauthenticated:
		httpStatus, code, message = http.StatusUnauthorized, "unauthenticated", "authentication required"
	case codes.PermissionDenied:
		httpStatus, code, message = http.StatusForbidden, "permission_denied", "permission denied"
	case codes.NotFound:
		httpStatus, code, message = http.StatusNotFound, "not_found", "resource not found"
	case codes.AlreadyExists:
		httpStatus, code, message = http.StatusConflict, "already_exists", "resource already exists"
	case codes.Aborted:
		httpStatus, code, message = http.StatusConflict, "aborted", "operation conflicted"
	case codes.FailedPrecondition:
		httpStatus, code, message = http.StatusConflict, "failed_precondition", "operation is not allowed in the current state"
	case codes.ResourceExhausted:
		if tooLarge {
			httpStatus, code, message = http.StatusRequestEntityTooLarge, "payload_too_large", "request payload is too large"
		} else {
			httpStatus, code, message = http.StatusTooManyRequests, "rate_limited", "too many requests"
		}
	case codes.Unavailable:
		httpStatus, code, message = http.StatusServiceUnavailable, "downstream_unavailable", "service unavailable"
	case codes.DeadlineExceeded:
		httpStatus, code, message = http.StatusGatewayTimeout, "deadline_exceeded", "downstream timeout"
	}

	writeError(w, httpStatus, code, message)
}

func isTooLargeGRPCError(message string) bool {
	value := strings.ToLower(message)
	return strings.Contains(value, "too large") ||
		strings.Contains(value, "size limit") ||
		strings.Contains(value, "larger than max") ||
		strings.Contains(value, "exceeds the configured size")
}
