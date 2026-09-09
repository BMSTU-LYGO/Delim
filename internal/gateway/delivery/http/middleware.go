package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"time"

	"delim/pkg/metricsx"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/metadata"
)

type requestIDContextKey struct{}

const (
	maxRequestIDLength = 128
	requestIDMetadata  = "x-request-id"
	requestIDHeader    = "X-Request-ID"
)

// readEntropy is the entropy source used to generate request ids. Tests may
// temporarily replace it via generateRequestIDWith to exercise the
// entropy-failure path deterministically without affecting production.
var readEntropy = rand.Read

// fallbackClock is the wall clock used only by the entropy-failure fallback.
// Tests may override it via generateRequestIDWith for determinism.
var fallbackClock = time.Now

// fallbackCounter is incremented for every entropy-failure fallback so that
// concurrent requests still receive distinct ids when crypto/rand fails.
var fallbackCounter atomic.Uint64

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := normalizeRequestID(r.Header.Get(requestIDHeader))
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, id)
		ctx = metadata.AppendToOutgoingContext(ctx, requestIDMetadata, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func normalizeRequestID(raw string) string {
	if isValidRequestID(raw) {
		return raw
	}
	return generateRequestID()
}

func generateRequestID() string {
	return generateRequestIDWith(func(value []byte) error {
		_, err := readEntropy(value)
		return err
	}, fallbackClock)
}

// generateRequestIDWith returns a safe, bounded, nonempty request id. On the
// normal path it returns a 32-character hex string from the supplied reader.
// When the reader fails (which should not happen on supported platforms) it
// returns a deterministic, bounded fallback that is explicitly NOT a
// security primitive and must never be used for authorization or identity.
func generateRequestIDWith(reader func([]byte) error, clock func() time.Time) string {
	var value [16]byte
	if err := reader(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fallbackRequestID(clock)
}

// fallbackRequestID is the bounded, safe, nonempty fallback used only when
// the entropy source is unavailable. The result is explicitly NOT a security
// primitive and must never be used for authorization or identity decisions.
func fallbackRequestID(clock func() time.Time) string {
	counter := fallbackCounter.Add(1)
	return fmt.Sprintf("fb-%016x-%016x", uint64(clock().UnixNano()), counter)
}

func isValidRequestID(value string) bool {
	if value == "" || len(value) > maxRequestIDLength {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

func recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error(
						"http panic",
						"request_id", requestIDFromContext(r.Context()),
						"operation", r.Method+" "+r.URL.Path,
						"status", http.StatusInternalServerError,
						"result", "error",
						"error_class", "panic",
						"stack", string(debug.Stack()),
					)
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// httpResult returns the normalized "success"/"error" result for an HTTP
// response status code.
func httpResult(status int) string {
	if status >= 500 {
		return "error"
	}
	if status >= 400 {
		return "error"
	}
	return "success"
}

func accessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			response := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(response, r)
			log.Info(
				"http request",
				"request_id", requestIDFromContext(r.Context()),
				"operation", r.Method+" "+r.URL.Path,
				"duration_ms", time.Since(started).Milliseconds(),
				"status", response.status,
				"result", httpResult(response.status),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

// Status exposes the captured status code so downstream middleware (notably
// the metrics middleware) can read it without re-wrapping the response.
func (w *responseWriter) Status() int {
	if w == nil {
		return http.StatusOK
	}
	return w.status
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

func cors(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Max-Bot-Api-Secret, X-Request-ID")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.Header().Add("Vary", "Origin")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeaders applies baseline defensive headers to every response.
// CSP is intentionally omitted: the Gateway serves JSON API responses only.
// The policy avoids breaking MAX WebView embeds (no frame/feature lockdown
// beyond what the WebView itself already controls).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		next.ServeHTTP(w, r)
	})
}

// noStore marks responses as non-cacheable. It is applied to auth/private
// API surfaces so sessions, balances, and document data never persist in
// shared or WebView caches.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// metricsMiddleware records request count, latency, and status class per
// completed HTTP request. The route label uses the chi route template (when
// available) so labels stay bounded and never include user-controlled URL
// fragments. The handler is a no-op when recorder is nil.
func metricsMiddleware(recorder *metricsx.Recorder) func(http.Handler) http.Handler {
	if recorder == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			next.ServeHTTP(w, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			status := 0
			if rw, ok := w.(interface{ Status() int }); ok {
				status = rw.Status()
			}
			if status == 0 {
				status = http.StatusOK
			}
			recorder.ObserveHTTP(r.Method, route, status, time.Since(started))
		})
	}
}
