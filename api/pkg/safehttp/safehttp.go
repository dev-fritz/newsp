// Package safehttp provides an HTTP client for URLs that come from outside
// (feeds, articles found in feeds). It refuses to connect to private, loopback,
// link-local (cloud metadata), multicast and unspecified addresses. The check
// runs on the resolved IP of every connection, including redirects, so DNS
// rebinding and redirects to internal hosts are blocked too.
package safehttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// UserAgent identifies newsp to the sites it reads.
const UserAgent = "newsp/0.1 (+https://github.com/dev-fritz/newsp)"

// ErrBlockedAddress is returned when a destination is not publicly routable.
var ErrBlockedAddress = errors.New("destination address is not allowed")

// ErrTooLarge is returned when a response body exceeds the read limit.
var ErrTooLarge = errors.New("response body too large")

// Shared address space (RFC 6598) is not flagged by netip helpers.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// Options tune the client.
type Options struct {
	Timeout      time.Duration // whole request, default 20s
	MaxRedirects int           // default 3
	// AllowPrivate disables the address check. Only for tests against httptest
	// servers on 127.0.0.1; never set it in production code.
	AllowPrivate bool
}

// NewClient returns an *http.Client hardened against SSRF.
func NewClient(opts Options) *http.Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 20 * time.Second
	}
	if opts.MaxRedirects <= 0 {
		opts.MaxRedirects = 3
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if !opts.AllowPrivate {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			return CheckAddress(address)
		}
	}
	transport := &http.Transport{
		Proxy:                 nil, // a proxy would bypass the address check
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= opts.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", opts.MaxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
}

// CheckAddress validates a dialed "ip:port".
func CheckAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, address)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, host)
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || cgnat.Contains(ip) {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, ip)
	}
	return nil
}

// Doer sends HTTP requests (an *http.Client).
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Get performs a GET with the newsp User-Agent and returns the body, refusing
// non-2xx statuses and bodies over limit bytes.
func Get(ctx context.Context, client Doer, url string, limit int64, header http.Header) ([]byte, error) {
	return send(ctx, client, http.MethodGet, url, nil, limit, header)
}

// Post sends body as JSON (unless header sets another Content-Type) and
// returns the response body under the same rules as Get.
func Post(ctx context.Context, client Doer, url string, body []byte, limit int64, header http.Header) ([]byte, error) {
	h := header.Clone()
	if h == nil {
		h = http.Header{}
	}
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", "application/json")
	}
	return send(ctx, client, http.MethodPost, url, body, limit, h)
}

func send(ctx context.Context, client Doer, method, url string, body []byte, limit int64, header http.Header) ([]byte, error) {
	var payload io.Reader
	if body != nil {
		payload = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, &StatusError{Code: res.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, ErrTooLarge
	}
	return data, nil
}

// StatusError is a non-2xx HTTP response.
type StatusError struct {
	Code int
}

func (e *StatusError) Error() string { return fmt.Sprintf("unexpected HTTP status %d", e.Code) }

// Temporary reports whether retrying later may succeed (429 and 5xx).
func (e *StatusError) Temporary() bool {
	return e.Code == http.StatusTooManyRequests || e.Code >= 500
}
