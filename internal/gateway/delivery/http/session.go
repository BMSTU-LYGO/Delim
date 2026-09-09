package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"delim/internal/gateway/auth"
	"google.golang.org/grpc/metadata"
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
			ctx = metadata.AppendToOutgoingContext(ctx, "x-user-id", strconv.FormatInt(session.UserID, 10))
			ctx = metadata.AppendToOutgoingContext(ctx, "x-max-user-id", strconv.FormatInt(session.MAXUserID, 10))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func userIDFromContext(ctx context.Context) (int64, bool) {
	session, ok := sessionFromContext(ctx)
	return session.UserID, ok
}

func maxUserIDFromContext(ctx context.Context) (int64, bool) {
	session, ok := sessionFromContext(ctx)
	return session.MAXUserID, ok
}

// chatIDFromContext returns the server-verified MAX chat captured at login.
// It never trusts a client-supplied chat id.
func chatIDFromContext(ctx context.Context) (int64, bool) {
	session, ok := sessionFromContext(ctx)
	if !ok || session.ChatID == 0 {
		return 0, false
	}
	return session.ChatID, true
}

func sessionFromContext(ctx context.Context) (auth.Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(auth.Session)
	return session, ok
}
