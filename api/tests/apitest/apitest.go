// Package apitest starts the real router against an isolated test database.
package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/bootstrap"
	"github.com/dev-fritz/newsp/api/internal/config"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/tests/testdb"
)

// Server is a running API with helpers for JSON calls.
type Server struct {
	t         *testing.T
	Container *bootstrap.Container
	srv       *httptest.Server
}

// New starts an API backed by a fresh database with the real AI clients.
func New(t *testing.T) *Server {
	t.Helper()
	return NewWithAI(t, nil)
}

// NewWithAI starts an API whose AI provider clients come from factory.
func NewWithAI(t *testing.T, factory services.ProviderFactory) *Server {
	t.Helper()
	return NewWithOptions(t, bootstrap.Options{AIFactory: factory})
}

// NewWithOptions starts an API with swapped dependencies, e.g. an HTTP client
// that may reach httptest feed servers on loopback.
func NewWithOptions(t *testing.T, opts bootstrap.Options) *Server {
	t.Helper()
	db := testdb.New(t)
	env := map[string]string{
		"APP_ENV":        "test",
		"DATABASE_URL":   "postgres://unused",
		"JWT_SECRET":     "integration-test-secret-with-32-bytes!!",
		"APP_BASE_URL":   "https://newsp.test",
		"ENCRYPTION_KEY": "dGVzdC1lbmNyeXB0aW9uLWtleS0zMi1ieXRlcyEhISE=",
	}
	cfg, err := config.Load(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	require.NoError(t, err)

	c, err := bootstrap.NewContainer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), db, opts)
	require.NoError(t, err)
	srv := httptest.NewServer(bootstrap.NewRouter(c))
	t.Cleanup(srv.Close)
	return &Server{t: t, Container: c, srv: srv}
}

// Response is a decoded API response.
type Response struct {
	Status int
	Header http.Header
	Body   map[string]any
}

// Data returns body["data"] as an object.
func (r Response) Data() map[string]any {
	d, _ := r.Body["data"].(map[string]any)
	return d
}

// List returns body["data"] as a list of objects.
func (r Response) List() []map[string]any {
	raw, _ := r.Body["data"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		m, _ := v.(map[string]any)
		out = append(out, m)
	}
	return out
}

// ErrorCode returns body["error"]["code"].
func (r Response) ErrorCode() string {
	e, _ := r.Body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

// Do sends a JSON request; token, when set, goes in the Authorization header.
func (s *Server) Do(method, path, token string, body any) Response {
	s.t.Helper()
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		require.NoError(s.t, err)
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(s.t.Context(), method, s.srv.URL+path, reader)
	require.NoError(s.t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := s.srv.Client().Do(req)
	require.NoError(s.t, err)
	defer func() { _ = res.Body.Close() }()

	out := Response{Status: res.StatusCode, Header: res.Header, Body: map[string]any{}}
	raw, err := io.ReadAll(res.Body)
	require.NoError(s.t, err)
	if len(raw) > 0 {
		require.NoError(s.t, json.Unmarshal(raw, &out.Body), "body: %s", raw)
	}
	return out
}
