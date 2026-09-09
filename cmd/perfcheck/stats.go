package main

import (
	"context"
	"io"
	"math"
	"net/http"
	"sort"
	"time"
)

type sample struct {
	duration time.Duration
	status   int
	err      error
}

type stats struct {
	median time.Duration
	p95    time.Duration
	errors int
}

// measure warms up (untimed) then runs the timed samples for a single
// operation. A transport failure during warmup is fatal (stack likely down);
// a non-2xx warmup is tolerated and only counted during timed samples.
func measure(ctx context.Context, client *http.Client, op operation, env *environment, iterations, warmup int) ([]sample, error) {
	for i := 0; i < warmup; i++ {
		if entry := fire(ctx, client, op, env); entry.err != nil {
			return nil, entry.err
		}
	}
	samples := make([]sample, 0, iterations)
	for i := 0; i < iterations; i++ {
		samples = append(samples, fire(ctx, client, op, env))
	}
	return samples, nil
}

func fire(ctx context.Context, client *http.Client, op operation, env *environment) sample {
	request, err := op.request(ctx, env)
	if err != nil {
		return sample{err: err}
	}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return sample{duration: time.Since(started), err: err}
	}
	drain(response)
	return sample{duration: time.Since(started), status: response.StatusCode}
}

func drain(response *http.Response) {
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
}

// summarize computes median and nearest-rank p95 and counts non-2xx or
// transport-failed samples as errors.
func summarize(samples []sample) stats {
	durations := make([]time.Duration, 0, len(samples))
	value := stats{}
	for _, entry := range samples {
		if entry.err != nil || entry.status < 200 || entry.status >= 300 {
			value.errors++
		}
		durations = append(durations, entry.duration)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	value.median = percentile(durations, 0.5)
	value.p95 = percentile(durations, 0.95)
	return value
}

// percentile returns the nearest-rank percentile of an ascending slice.
// q is a fraction in [0,1]; an empty slice yields zero.
func percentile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	rank := int(math.Ceil(float64(len(sorted)) * q))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
