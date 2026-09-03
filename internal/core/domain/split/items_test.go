package split

import "testing"

func TestItems(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		items   []Item
		want    []ItemAllocation
		wantErr bool
	}{
		{"valid", 101, []Item{{60, []int64{2, 1}}, {41, []int64{2}}}, []ItemAllocation{{0, 1, 30}, {0, 2, 30}, {1, 2, 41}}, false},
		{"item remainder", 5, []Item{{5, []int64{3, 1}}}, []ItemAllocation{{0, 1, 3}, {0, 3, 2}}, false},
		{"sum mismatch", 100, []Item{{99, []int64{1}}}, nil, true},
		{"no participants", 100, []Item{{100, nil}}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Items(tt.amount, tt.items)
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
