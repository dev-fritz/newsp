package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakePinger struct{ err error }

func (f fakePinger) PingContext(context.Context) error { return f.err }

func TestHealthHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ping       error
		handler    func(h *HealthHandler) http.HandlerFunc
		wantStatus int
	}{
		{name: "live", handler: func(h *HealthHandler) http.HandlerFunc { return h.Live }, wantStatus: http.StatusOK},
		{name: "ready", handler: func(h *HealthHandler) http.HandlerFunc { return h.Ready }, wantStatus: http.StatusOK},
		{name: "not ready", ping: errors.New("down"), handler: func(h *HealthHandler) http.HandlerFunc { return h.Ready }, wantStatus: http.StatusServiceUnavailable},
		{name: "live ignores database", ping: errors.New("down"), handler: func(h *HealthHandler) http.HandlerFunc { return h.Live }, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			tt.handler(NewHealthHandler(fakePinger{err: tt.ping}))(w, httptest.NewRequest(http.MethodGet, "/", nil))
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
