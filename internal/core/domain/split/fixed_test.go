package split

import "testing"

func TestFixed(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		values  []Allocation
		members []int64
		wantErr bool
	}{
		{"valid", 100, []Allocation{{2, 60}, {1, 40}}, []int64{1, 2, 3}, false},
		{"sum mismatch", 100, []Allocation{{1, 99}}, []int64{1}, true},
		{"negative", 100, []Allocation{{1, 110}, {2, -10}}, []int64{1, 2}, true},
		{"non member", 100, []Allocation{{2, 100}}, []int64{1}, true},
		{"duplicate", 100, []Allocation{{1, 50}, {1, 50}}, []int64{1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Fixed(tt.amount, tt.values, tt.members)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got %v, error %v", got, err)
			}
		})
	}
}
