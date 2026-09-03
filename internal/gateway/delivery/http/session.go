package http

import (
	"context"
	"net/http"
	"strings"

	"delim/internal/gateway/auth"
)

type sessionContextKey struct{}

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
			ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func maxUserIDFromContext(ctx context.Context) (int64, bool) {
	session, ok := sessionFromContext(ctx)
	return session.MAXUserID, ok
}

func sessionFromContext(ctx context.Context) (auth.Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(auth.Session)
	return session, ok
}
