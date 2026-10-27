package middlewares

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dev-fritz/newsp/api/internal/auth"
)

// TokenParser verifies access tokens.
type TokenParser interface {
	Parse(raw string) (auth.Identity, error)
}

// RequireAuth rejects requests without a valid Bearer access token and puts
// the caller identity on the context.
func RequireAuth(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
				return
			}
			id, err := parser.Parse(raw)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), id)))
		})
	}
}

// RequirePermission rejects authenticated callers lacking p. Services still
// check permissions themselves; this keeps whole route groups closed.
func RequirePermission(p auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := auth.FromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
				return
			}
			if !auth.Can(id, p) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "you do not have permission to do this")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Limiter is a keyed rate limiter.
type Limiter interface {
	Allow(key string) (bool, time.Duration)
}

// RateLimitByIP limits requests per client IP for an expensive or sensitive
// route group. ClientIP must run before it.
func RateLimitByIP(l Limiter, scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, wait := l.Allow(scope + ":" + ClientIPFrom(r.Context()))
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
				writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests, try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
