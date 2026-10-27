package middlewares

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestRecovererReturns500(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
	assert.NotContains(t, w.Body.String(), "boom")
}

func TestRequestLoggerUsesRouteTemplate(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r := chi.NewRouter()
	r.Use(RequestLogger(logger))
	r.Get("/reset/{token}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/reset/secret-token?x=1", nil))

	assert.Contains(t, buf.String(), "route=/reset/{token}")
	assert.NotContains(t, buf.String(), "secret-token")
	assert.Contains(t, buf.String(), "status=204")
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	dev := httptest.NewRecorder()
	SecurityHeaders(false)(ok).ServeHTTP(dev, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, "nosniff", dev.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, dev.Header().Get("Strict-Transport-Security"))

	prod := httptest.NewRecorder()
	SecurityHeaders(true)(ok).ServeHTTP(prod, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.NotEmpty(t, prod.Header().Get("Strict-Transport-Security"))
}
