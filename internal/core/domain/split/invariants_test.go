package split

import (
	"reflect"
	"testing"
)

func TestSplitInvariants(t *testing.T) {
	tests := []struct {
		name      string
		amount    int64
		calculate func() ([]Allocation, error)
	}{
		{"equal remainder", 10000, func() ([]Allocation, error) { return Equal(10000, []int64{7, 2, 5}) }},
		{"fixed", 10000, func() ([]Allocation, error) { return Fixed(10000, []Allocation{{7, 2500}, {2, 7500}}, []int64{2, 7}) }},
		{"shares remainder", 10001, func() ([]Allocation, error) { return Shares(10001, []Allocation{{7, 2}, {2, 1}}, []int64{2, 7}) }},
		{"percentage remainder", 10001, func() ([]Allocation, error) {
			return Percentage(10001, []Allocation{{7, 2500}, {2, 7500}}, []int64{2, 7})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := tt.calculate()
			if err != nil {
				t.Fatal(err)
			}
			second, err := tt.calculate()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("non-deterministic: %v != %v", first, second)
			}
			var total int64
			var previous int64
			for i, allocation := range first {
				total += allocation.AmountMinor
				if i > 0 && allocation.UserID < previous {
					t.Fatalf("not ordered by user_id: %v", first)
				}
				previous = allocation.UserID
			}
			if total != tt.amount {
				t.Fatalf("sum %d want %d", total, tt.amount)
			}
		})
	}
}

func TestItemSplitAllocationSum(t *testing.T) {
	allocations, err := Items(10001, []Item{{5001, []int64{7, 2}}, {5000, []int64{7, 5, 2}}})
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, allocation := range allocations {
		total += allocation.AmountMinor
	}
	if total != 10001 {
		t.Fatalf("sum %d want 10001", total)
	}
}
