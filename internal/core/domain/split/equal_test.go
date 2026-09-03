package split

import "testing"

func TestEqual(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		ids     []int64
		want    []Allocation
		wantErr bool
	}{
		{"exact", 100, []int64{2, 1}, []Allocation{{1, 50}, {2, 50}}, false},
		{"one minor unit", 1, []int64{3, 1, 2}, []Allocation{{1, 1}, {2, 0}, {3, 0}}, false},
		{"remainder", 10000, []int64{3, 1, 2}, []Allocation{{1, 3334}, {2, 3333}, {3, 3333}}, false},
		{"one participant", 17, []int64{9}, []Allocation{{9, 17}}, false},
		{"empty", 10, nil, nil, true},
		{"duplicate", 10, []int64{1, 1}, nil, true},
		{"invalid id", 10, []int64{0}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Equal(tt.amount, tt.ids)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v", err)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v want %v", got, tt.want)
				}
			}
		})
	}
}
