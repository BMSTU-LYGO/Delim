package split

import "testing"

func TestShares(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		shares  []Allocation
		members []int64
		want    []Allocation
		wantErr bool
	}{
		{"two to one", 30000, []Allocation{{1, 2}, {2, 1}}, []int64{1, 2}, []Allocation{{1, 20000}, {2, 10000}}, false},
		{"remainder", 10, []Allocation{{3, 1}, {1, 1}, {2, 1}}, []int64{1, 2, 3}, []Allocation{{1, 4}, {2, 3}, {3, 3}}, false},
		{"zero share", 10, []Allocation{{1, 0}}, []int64{1}, nil, true},
		{"duplicate", 10, []Allocation{{1, 1}, {1, 1}}, []int64{1}, nil, true},
		{"non member", 10, []Allocation{{2, 1}}, []int64{1}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Shares(tt.amount, tt.shares, tt.members)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v", err)
			}
			if tt.wantErr {
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v want %v", got, tt.want)
				}
			}
		})
	}
}
