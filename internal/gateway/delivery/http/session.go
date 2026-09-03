package http

import (
	"context"
	"net/http"
	"strings"

	"delim/internal/gateway/auth"
)

type maxUserIDContextKey struct{}

func sessionAuth(sessions *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const prefix = "Bearer "
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, prefix) || len(header) == len(prefix) || strings.Contains(header[len(prefix):], " ") {
				writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
				return
			}
			session, err := sessions.Verify(header[len(prefix):])
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
				return
			}
			ctx := context.WithValue(r.Context(), maxUserIDContextKey{}, session.MAXUserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func maxUserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(maxUserIDContextKey{}).(int64)
	return userID, ok
}
