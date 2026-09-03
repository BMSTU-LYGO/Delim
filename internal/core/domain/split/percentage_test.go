package split

import "testing"

func TestPercentage(t *testing.T) {
	tests := []struct {
		name        string
		amount      int64
		percentages []Allocation
		want        []Allocation
		wantErr     bool
	}{
		{"exact", 10000, []Allocation{{1, 1250}, {2, 8750}}, []Allocation{{1, 1250}, {2, 8750}}, false},
		{"remainder", 1, []Allocation{{2, 5000}, {1, 5000}}, []Allocation{{1, 1}, {2, 0}}, false},
		{"not 100 percent", 100, []Allocation{{1, 9999}}, nil, true},
		{"over 100 percent", 100, []Allocation{{1, 6000}, {2, 5000}}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			members := make([]int64, len(tt.percentages))
			for i, percentage := range tt.percentages {
				members[i] = percentage.UserID
			}
			got, err := Percentage(tt.amount, tt.percentages, members)
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
