package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
)

type fakeParser struct {
	id  auth.Identity
	err error
}

func (f fakeParser) Parse(string) (auth.Identity, error) { return f.id, f.err }

func TestRequireAuthAndPermission(t *testing.T) {
	t.Parallel()
	member := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	tests := []struct {
		name   string
		header string
		parser fakeParser
		want   int
	}{
		{name: "missing header", header: "", parser: fakeParser{id: admin}, want: http.StatusUnauthorized},
		{name: "not bearer", header: "Basic abc", parser: fakeParser{id: admin}, want: http.StatusUnauthorized},
		{name: "invalid token", header: "Bearer x", parser: fakeParser{err: errors.New("bad")}, want: http.StatusUnauthorized},
		{name: "member lacks permission", header: "Bearer x", parser: fakeParser{id: member}, want: http.StatusForbidden},
		{name: "admin allowed", header: "Bearer x", parser: fakeParser{id: admin}, want: http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := RequireAuth(tt.parser)(RequirePermission(auth.PermUsersManage)(ok))
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			assert.Equal(t, tt.want, w.Code)
		})
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}

	tests := []struct {
		name   string
		remote string
		xff    string
		want   string
	}{
		{name: "direct client ignores xff", remote: "203.0.113.7:4000", xff: "1.2.3.4", want: "203.0.113.7"},
		{name: "trusted proxy forwards", remote: "10.0.0.5:4000", xff: "198.51.100.9", want: "198.51.100.9"},
		{name: "spoofed leftmost hop ignored", remote: "10.0.0.5:4000", xff: "1.1.1.1, 198.51.100.9", want: "198.51.100.9"},
		{name: "chain of trusted proxies", remote: "10.0.0.5:4000", xff: "198.51.100.9, 10.0.0.6", want: "198.51.100.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got string
			h := ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = ClientIPFrom(r.Context()) }))
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			h.ServeHTTP(httptest.NewRecorder(), r)
			assert.Equal(t, tt.want, got)
		})
	}
}

type denyLimiter struct{}

func (denyLimiter) Allow(string) (bool, time.Duration) { return false, 1500 * time.Millisecond }

func TestRateLimitByIP(t *testing.T) {
	t.Parallel()
	h := RateLimitByIP(denyLimiter{}, "login")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "2", w.Header().Get("Retry-After"))
}
