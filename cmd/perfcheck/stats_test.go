package main

import (
	"errors"
	"testing"
	"time"
)

func durations(values ...int) []time.Duration {
	out := make([]time.Duration, 0, len(values))
	for _, value := range values {
		out = append(out, time.Duration(value)*time.Millisecond)
	}
	return out
}

func TestPercentileNearestRank(t *testing.T) {
	cases := []struct {
		name  string
		input []time.Duration
		q     float64
		want  time.Duration
	}{
		{"empty", nil, 0.95, 0},
		{"single", durations(12), 0.95, 12 * time.Millisecond},
		{"median odd", durations(1, 2, 3, 4, 5), 0.5, 3 * time.Millisecond},
		{"p95 of hundred", durations(seq(100)...), 0.95, 95 * time.Millisecond},
		{"p95 small", durations(10, 20, 30, 40), 0.95, 40 * time.Millisecond},
		{"clamps above one", durations(1, 2, 3), 1.5, 3 * time.Millisecond},
	}
	for _, entry := range cases {
		t.Run(entry.name, func(t *testing.T) {
			got := percentile(entry.input, entry.q)
			if got != entry.want {
				t.Fatalf("percentile = %v, want %v", got, entry.want)
			}
		})
	}
}

func TestSummarizeCountsNonSuccessAndTransportErrors(t *testing.T) {
	samples := []sample{
		{duration: 10 * time.Millisecond, status: 200},
		{duration: 20 * time.Millisecond, status: 201},
		{duration: 30 * time.Millisecond, status: 429},
		{duration: 40 * time.Millisecond, status: 503},
		{duration: 50 * time.Millisecond, err: errors.New("boom")},
	}
	value := summarize(samples)
	if value.errors != 3 {
		t.Fatalf("errors = %d, want 3", value.errors)
	}
	// sorted: 10,20,30,40,50 -> median rank ceil(5*0.5)=3 -> 30ms
	if value.median != 30*time.Millisecond {
		t.Fatalf("median = %v, want 30ms", value.median)
	}
	if value.p95 != 50*time.Millisecond {
		t.Fatalf("p95 = %v, want 50ms", value.p95)
	}
}

func seq(n int) []int {
	out := make([]int, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, i)
	}
	return out
}
