// Package metricsx provides a lightweight Prometheus-compatible metrics layer
// for Delim services. It implements the Prometheus text exposition format
// directly so the service has no extra runtime dependencies while remaining
// compatible with Prometheus and compatible scrapers.
//
// All metric names and label keys are intentionally bounded; user-controlled
// identifiers (user IDs, group IDs, receipt IDs, raw URLs, tokens, etc.) are
// never emitted as label values.
package metricsx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Config controls the internal metrics endpoint listener. An empty Host and a
// zero Port disables the metrics endpoint entirely; the returned Recorder is a
// safe no-op in that case.
type Config struct {
	Host string
	Port int
}

// Enabled reports whether the metrics endpoint should bind a listener.
func (c Config) Enabled() bool { return c.Port > 0 }

// Recorder owns the bounded metrics for a service. The zero value is not
// usable; obtain one via New or NoOp.
type Recorder struct {
	logger     *slog.Logger
	service    string
	mu         sync.RWMutex
	started    bool
	stopped    bool
	server     *http.Server
	listener   net.Listener
	counters   map[string]*counterVec
	histograms map[string]*histogramVec
}

// Options bundles the inputs required to build a Recorder.
type Options struct {
	// Service is the bounded service label emitted alongside every metric
	// (e.g. "gateway", "core", "document"). It must be a short, stable
	// identifier and never include user-controlled data.
	Service string
	// Logger is used to surface listener failures. Optional; falls back to
	// slog.Default().
	Logger *slog.Logger
}

// New builds a Recorder and binds an internal /metrics endpoint when Config
// is enabled. The returned shutdown function stops the endpoint on context
// cancellation or programmatically.
//
// Recorder construction itself never blocks. If the listener cannot bind the
// error is returned to the caller so it can fail fast at service startup.
func New(cfg Config, opts Options) (*Recorder, func(context.Context) error, error) {
	if opts.Service == "" {
		return nil, nil, errors.New("metricsx: service name is required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	recorder := &Recorder{
		logger:     logger,
		service:    opts.Service,
		counters:   make(map[string]*counterVec),
		histograms: make(map[string]*histogramVec),
	}

	if !cfg.Enabled() {
		return recorder, func(context.Context) error { return nil }, nil
	}

	address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, nil, fmt.Errorf("metricsx: bind %s: %w", address, err)
	}
	recorder.listener = listener
	recorder.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
			recorder.WriteText(w)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	recorder.started = true

	go func() {
		if err := recorder.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics endpoint terminated", "service", opts.Service, "error", err)
		}
	}()

	shutdown := func(context.Context) error { return recorder.shutdown() }
	return recorder, shutdown, nil
}

// NoOp returns a Recorder whose methods are safe to call but never record any
// values. Tests that do not assert on metrics should prefer this constructor.
func NoOp() *Recorder {
	return &Recorder{
		logger:     slog.Default(),
		service:    "noop",
		counters:   make(map[string]*counterVec),
		histograms: make(map[string]*histogramVec),
	}
}

// Service returns the bounded service label the recorder was constructed with.
func (r *Recorder) Service() string { return r.service }

// Handler exposes the Prometheus HTTP handler so tests can scrape the
// registry without binding a real listener. Returns nil when the recorder is
// nil.
func (r *Recorder) Handler() http.Handler {
	if r == nil {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.WriteText(w)
	})
}

// Shutdown releases the metrics endpoint and stops accepting new connections.
// Safe to call multiple times and from any goroutine.
func (r *Recorder) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.shutdown()
}

func (r *Recorder) shutdown() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return nil
	}
	r.stopped = true
	if r.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.server.Shutdown(ctx)
}

// counterVec is a bounded counter family with a fixed label set. Values are
// keyed by the joined label values in declaration order.
type counterVec struct {
	name   string
	help   string
	labels []string
	values map[string]*counterSample
}

func (v *counterVec) snapshot() []*counterSample {
	out := make([]*counterSample, 0, len(v.values))
	for _, sample := range v.values {
		out = append(out, sample)
	}
	return out
}

type counterSample struct {
	values []string
	value  float64
}

// histogramVec is a bounded histogram family. Buckets are inclusive upper
// bounds in seconds; the implicit +Inf bucket always exists.
type histogramVec struct {
	name    string
	help    string
	labels  []string
	buckets []float64
	values  map[string]*histogramSample
}

type histogramSample struct {
	values  []string
	buckets []bucketState
	sum     float64
	count   uint64
}

type bucketState struct {
	upperBound float64
	count      uint64
}

// counterWithLabels resolves or creates a counter sample for the given label
// values. Returns nil when r is nil.
func (r *Recorder) counterWithLabels(name, help string, labelNames []string, labelValues []string) *counterSample {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	vec, ok := r.counters[name]
	if !ok {
		vec = &counterVec{name: name, help: help, labels: append([]string(nil), labelNames...), values: make(map[string]*counterSample)}
		r.counters[name] = vec
	}
	key := strings.Join(labelValues, "\x00")
	sample, ok := vec.values[key]
	if !ok {
		sample = &counterSample{values: append([]string(nil), labelValues...)}
		vec.values[key] = sample
	}
	return sample
}

func (r *Recorder) histogramWithLabels(name, help string, labelNames []string, labelValues []string, buckets []float64) *histogramSample {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	vec, ok := r.histograms[name]
	if !ok {
		vec = &histogramVec{
			name:    name,
			help:    help,
			labels:  append([]string(nil), labelNames...),
			buckets: append([]float64(nil), buckets...),
			values:  make(map[string]*histogramSample),
		}
		// initialise +Inf bucket
		vec.buckets = append(vec.buckets, bucketInf)
		r.histograms[name] = vec
	}
	key := strings.Join(labelValues, "\x00")
	sample, ok := vec.values[key]
	if !ok {
		bs := make([]bucketState, len(vec.buckets))
		for i, b := range vec.buckets {
			bs[i] = bucketState{upperBound: b}
		}
		sample = &histogramSample{values: append([]string(nil), labelValues...), buckets: bs}
		vec.values[key] = sample
	}
	return sample
}

const bucketInf = -1

// HTTPStatusClass collapses an HTTP status code into a bounded class.
func HTTPStatusClass(status int) string {
	switch {
	case status < 200:
		return "1xx"
	case status < 300:
		return "2xx"
	case status < 400:
		return "3xx"
	case status < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// HTTPResult normalises an HTTP status into a bounded success/error label.
func HTTPResult(status int) string {
	if status >= 400 {
		return "error"
	}
	return "success"
}

// GRPCResult normalises a gRPC outcome into a bounded success/error label.
func GRPCResult(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}

// GRPCCode maps an error to its bounded gRPC code label.
func GRPCCode(err error) string {
	if err == nil {
		return "ok"
	}
	return grpcCodeFromError(err)
}

// ObserveHTTP records the duration and counters for a completed HTTP request.
func (r *Recorder) ObserveHTTP(method, route string, status int, duration time.Duration) {
	if r == nil {
		return
	}
	class := HTTPStatusClass(status)
	result := HTTPResult(status)
	r.incCounter(
		"delim_http_requests_total",
		"Total HTTP requests handled by the service.",
		[]string{"method", "route", "status_class", "result"},
		[]string{method, route, class, result},
	)
	r.observeHistogram(
		"delim_http_request_duration_seconds",
		"HTTP request duration in seconds.",
		[]string{"method", "route", "status_class"},
		[]string{method, route, class},
		defaultBuckets(),
		duration.Seconds(),
	)
}

// ObserveGRPCServer records the duration and counters for a completed gRPC
// server request.
func (r *Recorder) ObserveGRPCServer(method string, err error, duration time.Duration) {
	if r == nil {
		return
	}
	result := GRPCResult(err)
	code := GRPCCode(err)
	r.incCounter(
		"delim_grpc_requests_total",
		"Total gRPC requests handled by the service.",
		[]string{"method", "result", "code"},
		[]string{method, result, code},
	)
	r.observeHistogram(
		"delim_grpc_request_duration_seconds",
		"gRPC request duration in seconds.",
		[]string{"method"},
		[]string{method},
		defaultBuckets(),
		duration.Seconds(),
	)
}

// ObserveGRPCClientError records a client-side gRPC failure.
func (r *Recorder) ObserveGRPCClientError(peer, operation string, err error) {
	if r == nil {
		return
	}
	result := GRPCResult(err)
	r.incCounter(
		"delim_grpc_client_errors_total",
		"Total gRPC client errors grouped by peer/operation/result.",
		[]string{"peer", "operation", "result"},
		[]string{peer, operation, result},
	)
}

// ObserveMAXAPIError records a MAX API call failure.
func (r *Recorder) ObserveMAXAPIError(operation, statusClass string, err error) {
	if r == nil {
		return
	}
	result := GRPCResult(err)
	r.incCounter(
		"delim_max_api_errors_total",
		"Total MAX API client errors grouped by operation/status_class/result.",
		[]string{"operation", "status_class", "result"},
		[]string{operation, statusClass, result},
	)
}

// ObserveWebhook records a MAX webhook lifecycle event. result must be one of
// "accepted", "duplicate", "failed", or "ignored".
func (r *Recorder) ObserveWebhook(result string) {
	if r == nil {
		return
	}
	switch result {
	case "accepted", "duplicate", "failed", "ignored":
	default:
		result = "ignored"
	}
	r.incCounter(
		"delim_webhook_events_total",
		"Total MAX webhook events grouped by result.",
		[]string{"result"},
		[]string{result},
	)
}

// ObserveConflict records a business conflict (e.g. version mismatch).
func (r *Recorder) ObserveConflict(operation string) {
	if r == nil {
		return
	}
	r.incCounter(
		"delim_conflicts_total",
		"Total business conflicts grouped by operation.",
		[]string{"operation"},
		[]string{operation},
	)
}

// ObserveFinancialError records an error raised during a financial operation.
func (r *Recorder) ObserveFinancialError(operation, code string) {
	if r == nil {
		return
	}
	r.incCounter(
		"delim_financial_errors_total",
		"Total financial operation errors grouped by operation/code.",
		[]string{"operation", "code"},
		[]string{operation, code},
	)
}

// ObserveOCRJob records an OCR job terminal state.
func (r *Recorder) ObserveOCRJob(state string) {
	if r == nil {
		return
	}
	switch state {
	case "pending", "processing", "completed", "failed":
	default:
		state = "failed"
	}
	r.incCounter(
		"delim_ocr_jobs_total",
		"Total OCR job terminal events grouped by state.",
		[]string{"state"},
		[]string{state},
	)
}

// ObserveOCRDuration records the OCR processing duration.
func (r *Recorder) ObserveOCRDuration(result string, duration time.Duration) {
	if r == nil {
		return
	}
	if result != "completed" && result != "failed" {
		result = "failed"
	}
	r.observeHistogram(
		"delim_ocr_duration_seconds",
		"OCR processing duration in seconds.",
		[]string{"result"},
		[]string{result},
		[]float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		duration.Seconds(),
	)
}

// ObserveExportJob records an export job outcome.
func (r *Recorder) ObserveExportJob(format, result string) {
	if r == nil {
		return
	}
	format = normaliseExportFormat(format)
	if result != "success" && result != "error" {
		result = "error"
	}
	r.incCounter(
		"delim_export_jobs_total",
		"Total export jobs grouped by format/result.",
		[]string{"format", "result"},
		[]string{format, result},
	)
}

func normaliseExportFormat(value string) string {
	switch strings.ToLower(value) {
	case "csv", "pdf", "xlsx":
		return strings.ToLower(value)
	default:
		return "unspecified"
	}
}

// ObserveMinIOError records a MinIO error.
func (r *Recorder) ObserveMinIOError(operation string, err error) {
	if r == nil {
		return
	}
	result := GRPCResult(err)
	r.incCounter(
		"delim_minio_errors_total",
		"Total MinIO errors grouped by operation/result.",
		[]string{"operation", "result"},
		[]string{operation, result},
	)
}

func (r *Recorder) incCounter(name, help string, labelNames, labelValues []string) {
	sample := r.counterWithLabels(name, help, labelNames, labelValues)
	if sample == nil {
		return
	}
	sample.value++
}

func (r *Recorder) observeHistogram(name, help string, labelNames, labelValues []string, buckets []float64, value float64) {
	sample := r.histogramWithLabels(name, help, labelNames, labelValues, buckets)
	if sample == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	sample.sum += value
	sample.count++
	for i := range sample.buckets {
		bound := sample.buckets[i].upperBound
		if bound == bucketInf || value <= bound {
			sample.buckets[i].count++
		}
	}
}

func defaultBuckets() []float64 {
	return []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
}

// WriteText serialises the recorder state into the Prometheus text exposition
// format (version 0.0.4). Safe to call concurrently.
func (r *Recorder) WriteText(w io.Writer) {
	if r == nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Sorted metric names for stable output.
	counterNames := make([]string, 0, len(r.counters))
	for name := range r.counters {
		counterNames = append(counterNames, name)
	}
	sort.Strings(counterNames)
	for _, name := range counterNames {
		vec := r.counters[name]
		fmt.Fprintf(w, "# HELP %s %s\n", vec.name, escapeHelp(vec.help))
		fmt.Fprintf(w, "# TYPE %s counter\n", vec.name)
		samples := vec.snapshot()
		sort.Slice(samples, func(i, j int) bool { return sampleKey(samples[i].values) < sampleKey(samples[j].values) })
		for _, sample := range samples {
			fmt.Fprintf(w, "%s%s %s\n", vec.name, formatLabels(vec.labels, sample.values), formatFloat(sample.value))
		}
	}

	histogramNames := make([]string, 0, len(r.histograms))
	for name := range r.histograms {
		histogramNames = append(histogramNames, name)
	}
	sort.Strings(histogramNames)
	for _, name := range histogramNames {
		vec := r.histograms[name]
		fmt.Fprintf(w, "# HELP %s %s\n", vec.name, escapeHelp(vec.help))
		fmt.Fprintf(w, "# TYPE %s histogram\n", vec.name)
		samples := make([]*histogramSample, 0, len(vec.values))
		for _, s := range vec.values {
			samples = append(samples, s)
		}
		sort.Slice(samples, func(i, j int) bool { return sampleKey(samples[i].values) < sampleKey(samples[j].values) })
		for _, sample := range samples {
			labels := formatLabels(vec.labels, sample.values)
			for _, bucket := range sample.buckets {
				upper := "+Inf"
				if bucket.upperBound != bucketInf {
					upper = formatFloat(bucket.upperBound)
				}
				fmt.Fprintf(w, "%s_bucket%s %d\n", vec.name, withExtraLabel(labels, "le", upper), bucket.count)
			}
			fmt.Fprintf(w, "%s_sum%s %s\n", vec.name, labels, formatFloat(sample.sum))
			fmt.Fprintf(w, "%s_count%s %d\n", vec.name, labels, sample.count)
		}
	}
}

func escapeHelp(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, `\`, `\\`), "\n", `\n`), `"`, `\"`)
}

func formatLabels(names, values []string) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, name := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(name)
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(values[i]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func withExtraLabel(labels, name, value string) string {
	if labels == "" {
		return fmt.Sprintf(`{%s="%s"}`, name, value)
	}
	return fmt.Sprintf(`{%s,%s="%s"}`, strings.TrimSuffix(strings.TrimPrefix(labels, "{"), "}"), name, value)
}

func escapeLabelValue(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), "\n", `\n`), `"`, `\"`)
}

func formatFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", value), "0"), ".")
}

func sampleKey(values []string) string {
	return strings.Join(values, "\x00")
}
