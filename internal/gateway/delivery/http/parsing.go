package http

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

var errInvalidParameter = errors.New("invalid parameter")

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidParameter
	}
	return id, nil
}

func parseCursor(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cursor < 0 {
		return 0, errInvalidParameter
	}
	return cursor, nil
}

func parseLimit(value string) (int32, error) {
	if value == "" {
		return defaultPageLimit, nil
	}
	limit, err := strconv.ParseInt(value, 10, 32)
	if err != nil || limit <= 0 || limit > maxPageLimit {
		return 0, errInvalidParameter
	}
	return int32(limit), nil
}

func parseTimestamp(value string) (*timestamppb.Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, errInvalidParameter
	}
	return timestamppb.New(parsed), nil
}

func normalizeCurrency(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
