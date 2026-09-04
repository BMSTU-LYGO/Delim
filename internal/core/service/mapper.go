package service

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func timeToProto(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}
