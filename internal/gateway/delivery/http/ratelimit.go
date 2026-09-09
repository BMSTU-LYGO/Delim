package http

import (
	"math"
	"net"
	"net/http"
	"strconv"

	"delim/internal/gateway/ratelimit"
)

// RateLimits bundles the independently configured limiters for the public
// Gateway surface. A nil limiter disables the corresponding check.
type RateLimits struct {
	Auth    *ratelimit.Limiter // POST /api/v1/auth/max, keyed by client IP
	Webhook *ratelimit.Limiter // POST /api/v1/max/webhook, keyed by client IP
	Upload  *ratelimit.Limiter // receipt upload, keyed by authenticated user
	API     *ratelimit.Limiter // remaining authenticated API, keyed by user
}

// clientIPKey resolves the rate-limit identity for unauthenticated routes.
// It intentionally uses the transport peer address only: user-controlled
// headers such as X-Forwarded-For are never trusted as identity.
func clientIPKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// sessionUserKey resolves the rate-limit identity for authenticated routes
// from the verified session (Core user id). It falls back to the client IP
// only when no session is present (should not happen behind sessionAuth).
func sessionUserKey(r *http.Request) string {
	if userID, ok := userIDFromContext(r.Context()); ok {
		return "u:" + strconv.FormatInt(userID, 10)
	}
	return clientIPKey(r)
}

func rateLimit(limiter *ratelimit.Limiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	if limiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retryAfter := limiter.Allow(keyFunc(r))
			if !ok {
				seconds := int64(math.Ceil(retryAfter.Seconds()))
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
