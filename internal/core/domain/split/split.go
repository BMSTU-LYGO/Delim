package split

import (
	"delim/internal/core/domain"
	"sort"
)

type Allocation struct{ UserID, AmountMinor int64 }

func Equal(amountMinor int64, participantIDs []int64) ([]Allocation, error) {
	ids, err := validUniqueIDs(participantIDs)
	if err != nil || amountMinor <= 0 {
		return nil, domain.ErrInvalidArgument
	}
	base := amountMinor / int64(len(ids))
	remainder := amountMinor % int64(len(ids))
	result := make([]Allocation, len(ids))
	for i, id := range ids {
		result[i] = Allocation{UserID: id, AmountMinor: base}
		if int64(i) < remainder {
			result[i].AmountMinor++
		}
	}
	return result, nil
}

func Fixed(amountMinor int64, values []Allocation, participantIDs []int64) ([]Allocation, error) {
	allowedIDs, err := validUniqueIDs(participantIDs)
	if err != nil || amountMinor <= 0 || len(values) == 0 {
		return nil, domain.ErrInvalidArgument
	}
	allowed := make(map[int64]struct{}, len(allowedIDs))
	for _, id := range allowedIDs {
		allowed[id] = struct{}{}
	}
	result := append([]Allocation(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].UserID < result[j].UserID })
	var total int64
	for i, value := range result {
		if value.AmountMinor < 0 {
			return nil, domain.ErrInvalidArgument
		}
		if _, ok := allowed[value.UserID]; !ok {
			return nil, domain.ErrInvalidArgument
		}
		if i > 0 && value.UserID == result[i-1].UserID {
			return nil, domain.ErrInvalidArgument
		}
		if value.AmountMinor > amountMinor-total {
			return nil, domain.ErrInvalidArgument
		}
		total += value.AmountMinor
	}
	if total != amountMinor {
		return nil, domain.ErrInvalidArgument
	}
	return result, nil
}

func validUniqueIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, domain.ErrInvalidArgument
	}
	result := append([]int64(nil), ids...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	for i, id := range result {
		if id <= 0 || (i > 0 && id == result[i-1]) {
			return nil, domain.ErrInvalidArgument
		}
	}
	return result, nil
}
