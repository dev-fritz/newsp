package safehttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAddress(t *testing.T) {
	t.Parallel()
	blocked := []string{
		"127.0.0.1:80", "[::1]:443", "10.1.2.3:80", "172.16.0.1:80", "192.168.1.10:80",
		"169.254.169.254:80", "100.64.0.1:80", "0.0.0.0:80", "[fe80::1]:80", "[::ffff:127.0.0.1]:80",
		"[fd00::1]:80", "224.0.0.1:80",
	}
	for _, addr := range blocked {
		assert.ErrorIs(t, CheckAddress(addr), ErrBlockedAddress, addr)
	}
	for _, addr := range []string{"93.184.216.34:443", "[2606:2800:220:1:248:1893:25c8:1946]:443"} {
		assert.NoError(t, CheckAddress(addr), addr)
	}
}

func TestClientBlocksLoopback(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer srv.Close()

	_, err := Get(context.Background(), NewClient(Options{}), srv.URL, 1024, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBlockedAddress)
}

func TestGetLimitsAndStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, UserAgent, r.Header.Get("User-Agent"))
		switch r.URL.Path {
		case "/big":
			_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
		case "/limited":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	client := NewClient(Options{AllowPrivate: true})
	ctx := context.Background()

	body, err := Get(ctx, client, srv.URL+"/", 1024, nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))

	_, err = Get(ctx, client, srv.URL+"/big", 1024, nil)
	assert.ErrorIs(t, err, ErrTooLarge)

	_, err = Get(ctx, client, srv.URL+"/limited", 1024, nil)
	var statusErr *StatusError
	require.True(t, errors.As(err, &statusErr))
	assert.True(t, statusErr.Temporary())
}

func TestPostSendsJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, UserAgent, r.Header.Get("User-Agent"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer t", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"q":1}` {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	client := NewClient(Options{AllowPrivate: true})
	header := http.Header{"Authorization": {"Bearer t"}}

	body, err := Post(context.Background(), client, srv.URL, []byte(`{"q":1}`), 1024, header)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	assert.Empty(t, header.Get("Content-Type"), "the caller's header is not modified")

	_, err = Post(context.Background(), client, srv.URL, []byte(`{}`), 1024, header)
	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusUnauthorized, statusErr.Code)
}
