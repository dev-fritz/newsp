package connectors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const blueskyTestPassword = "abcd-efgh-ijkl-mnop"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return body
}

func TestBlueskyAuthorFetch(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "bluesky_feed.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/xrpc/app.bsky.feed.getAuthorFeed", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "builder.example.com", q.Get("actor"))
		assert.Equal(t, "10", q.Get("limit"))
		assert.Equal(t, "posts_no_replies", q.Get("filter"))
		assert.Empty(t, r.Header.Get("Authorization"))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := NewBluesky(srv.Client(), srv.URL, "", time.Now)
	got, err := c.Fetch(context.Background(), json.RawMessage(`{"actor":"@Builder.Example.com","limit":10}`), "")
	require.NoError(t, err)
	require.Len(t, got, 2, "the repost and the empty post are skipped")

	first := got[0]
	assert.Equal(t, "at://did:plc:aaaaaaaaaaaaaaaaaaaaaaaa/app.bsky.feed.post/3kaaaaaaaaaa2", first.ExternalID)
	assert.Equal(t, "https://bsky.app/profile/builder.example.com/post/3kaaaaaaaaaa2", first.URL)
	assert.Equal(t, "Shipped the invoicing beta today: 40 signups in the first hour.", first.Title)
	assert.Equal(t, "Shipped the invoicing beta today: 40 signups in the first hour. — Invoicing beta notes", first.Summary)
	assert.Equal(t, "builder.example.com", first.Author)
	assert.Equal(t, time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC), first.PublishedAt)
	assert.Equal(t, 31, *first.Score)
	assert.Equal(t, 4, *first.Comments)
	assert.Equal(t, map[string]any{"reposts": 7, "quotes": 2, "link_url": "https://example.com/invoicing-beta"}, first.Metrics)

	second := got[1]
	assert.Equal(t, time.Date(2026, 10, 24, 12, 30, 0, 0, time.UTC), second.PublishedAt, "indexedAt is the fallback")
	assert.NotContains(t, second.Metrics, "link_url")
}

func TestBlueskyFeedWithHandleResolvesFirst(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "bluesky_feed.json")
	var order []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, r.URL.Path)
		switch r.URL.Path {
		case "/xrpc/com.atproto.identity.resolveHandle":
			assert.Equal(t, "curator.example.com", r.URL.Query().Get("handle"))
			_, _ = w.Write([]byte(`{"did":"did:plc:eeeeeeeeeeeeeeeeeeeeeeee"}`))
		case "/xrpc/app.bsky.feed.getFeed":
			assert.Equal(t, "at://did:plc:eeeeeeeeeeeeeeeeeeeeeeee/app.bsky.feed.generator/startups", r.URL.Query().Get("feed"))
			assert.Equal(t, "30", r.URL.Query().Get("limit"))
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewBluesky(srv.Client(), srv.URL, "", time.Now)
	got, err := c.Fetch(context.Background(), json.RawMessage(`{"mode":"feed","feed":"https://bsky.app/profile/curator.example.com/feed/startups"}`), "")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, []string{"/xrpc/com.atproto.identity.resolveHandle", "/xrpc/app.bsky.feed.getFeed"}, order)
}

func TestBlueskyFeedWithDIDSkipsResolve(t *testing.T) {
	t.Parallel()
	body := readFixture(t, "bluesky_feed.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/app.bsky.feed.getFeed" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		assert.Equal(t, "at://did:plc:eeeeeeeeeeeeeeeeeeeeeeee/app.bsky.feed.generator/startups", r.URL.Query().Get("feed"))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := NewBluesky(srv.Client(), srv.URL, "", time.Now)
	got, err := c.Fetch(context.Background(), json.RawMessage(`{"mode":"feed","feed":"at://did:plc:eeeeeeeeeeeeeeeeeeeeeeee/app.bsky.feed.generator/startups"}`), "")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

// fakeClock is a manually advanced clock.
type fakeClock struct{ t atomic.Int64 }

func newFakeClock() *fakeClock {
	c := &fakeClock{}
	c.t.Store(time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC).UnixNano())
	return c
}
func (c *fakeClock) now() time.Time          { return time.Unix(0, c.t.Load()).UTC() }
func (c *fakeClock) advance(d time.Duration) { c.t.Add(int64(d)) }

// blueskyServer fakes the PDS: createSession and searchPosts.
type blueskyServer struct {
	*httptest.Server
	logins      atomic.Int32
	searches    atomic.Int32
	rejectNext  atomic.Bool // answer the next search with 401
	loginStatus int
}

func newBlueskyServer(t *testing.T, search []byte) *blueskyServer {
	t.Helper()
	s := &blueskyServer{loginStatus: http.StatusOK}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			n := s.logins.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Empty(t, r.URL.RawQuery, "the password never goes in the URL")
			raw, _ := io.ReadAll(r.Body)
			var in map[string]string
			assert.NoError(t, json.Unmarshal(raw, &in))
			assert.Equal(t, "me@example.com", in["identifier"])
			assert.Equal(t, blueskyTestPassword, in["password"])
			if s.loginStatus != http.StatusOK {
				w.WriteHeader(s.loginStatus)
				_, _ = w.Write([]byte(`{"error":"AuthenticationRequired","message":"Invalid identifier or password"}`))
				return
			}
			_, _ = w.Write([]byte(`{"accessJwt":"jwt-` + strconv.Itoa(int(n)) + `","did":"did:plc:ffffffffffffffffffffffff"}`))
		case "/xrpc/app.bsky.feed.searchPosts":
			s.searches.Add(1)
			assert.NotContains(t, r.URL.RawQuery, blueskyTestPassword)
			q := r.URL.Query()
			assert.Equal(t, "invoice tool", q.Get("q"))
			assert.Equal(t, "20", q.Get("limit"))
			assert.Equal(t, "latest", q.Get("sort"))
			if s.rejectNext.CompareAndSwap(true, false) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write(search)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	return s
}

const blueskySearchConfig = `{"mode":"search","query":" invoice tool ","sort":"latest","identifier":"me@example.com","limit":20}`

func TestBlueskySearchSessionCache(t *testing.T) {
	t.Parallel()
	srv := newBlueskyServer(t, readFixture(t, "bluesky_search.json"))
	defer srv.Close()
	clock := newFakeClock()
	c := NewBluesky(srv.Client(), "", srv.URL, clock.now)
	ctx := context.Background()

	got, err := c.Fetch(ctx, json.RawMessage(blueskySearchConfig), blueskyTestPassword)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "https://bsky.app/profile/founder.example.net/post/3kcccccccccc2", got[0].URL)
	assert.Equal(t, "Looking for a simple way to track freelance invoices. Any tool recommendations?", got[0].Summary)
	assert.Equal(t, 25, *got[0].Score)
	assert.Equal(t, 12, *got[0].Comments)
	assert.Equal(t, map[string]any{"reposts": 3, "quotes": 1}, got[0].Metrics)
	assert.NotContains(t, got[1].Metrics, "link_url", "non-http links are not exposed")
	assert.EqualValues(t, 1, srv.logins.Load())

	_, err = c.Fetch(ctx, json.RawMessage(blueskySearchConfig), blueskyTestPassword)
	require.NoError(t, err)
	assert.EqualValues(t, 1, srv.logins.Load(), "the second fetch reuses the session")
	assert.EqualValues(t, 2, srv.searches.Load())

	clock.advance(91 * time.Minute)
	_, err = c.Fetch(ctx, json.RawMessage(blueskySearchConfig), blueskyTestPassword)
	require.NoError(t, err)
	assert.EqualValues(t, 2, srv.logins.Load(), "an expired session logs in again")
}

func TestBlueskySearchSendsBearerToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			_, _ = w.Write([]byte(`{"accessJwt":"token-123","did":"did:plc:ffffffffffffffffffffffff"}`))
		case "/xrpc/app.bsky.feed.searchPosts":
			assert.Equal(t, "Bearer token-123", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"posts":[]}`))
		}
	}))
	defer srv.Close()
	c := NewBluesky(srv.Client(), "", srv.URL, time.Now)
	got, err := c.Fetch(context.Background(), json.RawMessage(`{"mode":"search","query":"x","identifier":"me@example.com"}`), blueskyTestPassword)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestBlueskySearchChangedPasswordLogsInAgain(t *testing.T) {
	t.Parallel()
	var logins atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xrpc/com.atproto.server.createSession" {
			logins.Add(1)
			_, _ = w.Write([]byte(`{"accessJwt":"t"}`))
			return
		}
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}))
	defer srv.Close()
	c := NewBluesky(srv.Client(), "", srv.URL, time.Now)
	cfg := json.RawMessage(`{"mode":"search","query":"x","identifier":"me@example.com"}`)
	_, err := c.Fetch(context.Background(), cfg, "old-password")
	require.NoError(t, err)
	_, err = c.Fetch(context.Background(), cfg, "new-password")
	require.NoError(t, err)
	assert.EqualValues(t, 2, logins.Load())
}

func TestBlueskySearchRetriesOnceOnUnauthorized(t *testing.T) {
	t.Parallel()
	srv := newBlueskyServer(t, readFixture(t, "bluesky_search.json"))
	defer srv.Close()
	c := NewBluesky(srv.Client(), "", srv.URL, newFakeClock().now)
	ctx := context.Background()

	_, err := c.Fetch(ctx, json.RawMessage(blueskySearchConfig), blueskyTestPassword)
	require.NoError(t, err)
	require.EqualValues(t, 1, srv.logins.Load())

	srv.rejectNext.Store(true)
	got, err := c.Fetch(ctx, json.RawMessage(blueskySearchConfig), blueskyTestPassword)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.EqualValues(t, 2, srv.logins.Load(), "the 401 dropped the session and a fresh login was made")
	assert.EqualValues(t, 3, srv.searches.Load(), "exactly one retry")
}

func TestBlueskySearchGivesUpAfterSecondUnauthorized(t *testing.T) {
	t.Parallel()
	var searches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xrpc/com.atproto.server.createSession" {
			_, _ = w.Write([]byte(`{"accessJwt":"t"}`))
			return
		}
		searches.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := NewBluesky(srv.Client(), "", srv.URL, time.Now)
	_, err := c.Fetch(context.Background(), json.RawMessage(`{"mode":"search","query":"x","identifier":"me@example.com"}`), blueskyTestPassword)
	require.Error(t, err)
	assert.EqualValues(t, 2, searches.Load())
	assert.NotContains(t, err.Error(), blueskyTestPassword)
}

func TestBlueskySearchWithoutSecret(t *testing.T) {
	t.Parallel()
	c := NewBluesky(http.DefaultClient, "http://127.0.0.1:1", "http://127.0.0.1:1", time.Now)
	_, err := c.Fetch(context.Background(), json.RawMessage(`{"mode":"search","query":"x","identifier":"me@example.com"}`), "")
	require.Error(t, err)
	assert.EqualError(t, err, "bluesky search needs the app password of me@example.com")
}

func TestBlueskyLoginFailureDoesNotLeakPassword(t *testing.T) {
	t.Parallel()
	srv := newBlueskyServer(t, nil)
	defer srv.Close()
	srv.loginStatus = http.StatusUnauthorized
	c := NewBluesky(srv.Client(), "", srv.URL, time.Now)
	_, err := c.Fetch(context.Background(), json.RawMessage(blueskySearchConfig), blueskyTestPassword)
	require.Error(t, err)
	assert.EqualError(t, err, "bluesky login failed for me@example.com: check the app password")
	assert.NotContains(t, err.Error(), blueskyTestPassword)
	assert.NotContains(t, err.Error(), "AuthenticationRequired", "the response body is not echoed")
	assert.EqualValues(t, 0, srv.searches.Load())
}

func TestBlueskyUpstreamError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := NewBluesky(srv.Client(), srv.URL, "", time.Now).Fetch(context.Background(), json.RawMessage(`{"actor":"builder.example.com"}`), "")
	require.Error(t, err)
	assert.False(t, IsConfigError(err))
	assert.Contains(t, err.Error(), "429")
}

func TestBlueskyNormalize(t *testing.T) {
	t.Parallel()
	c := NewBluesky(http.DefaultClient, "", "", time.Now)
	assert.Equal(t, "bluesky", c.Kind())

	valid := []struct{ name, in, want string }{
		{"author defaults", `{"actor":"alice.bsky.social"}`, `{"mode":"author","actor":"alice.bsky.social","feed":"","query":"","sort":"","identifier":"","limit":30}`},
		{"actor is cleaned", `{"actor":" @Alice.Bsky.Social "}`, `{"mode":"author","actor":"alice.bsky.social","feed":"","query":"","sort":"","identifier":"","limit":30}`},
		{"actor as did:plc", `{"mode":"author","actor":"did:plc:abc123xyz","limit":100}`, `{"mode":"author","actor":"did:plc:abc123xyz","feed":"","query":"","sort":"","identifier":"","limit":100}`},
		{"actor as did:web", `{"actor":"did:web:example.com"}`, `{"mode":"author","actor":"did:web:example.com","feed":"","query":"","sort":"","identifier":"","limit":30}`},
		{"feed at uri", `{"mode":"feed","feed":"at://did:plc:abc123/app.bsky.feed.generator/whats-hot"}`, `{"mode":"feed","actor":"","feed":"at://did:plc:abc123/app.bsky.feed.generator/whats-hot","query":"","sort":"","identifier":"","limit":30}`},
		{"feed web url with handle", `{"mode":"feed","feed":"https://bsky.app/profile/Curator.Example.com/feed/startups"}`, `{"mode":"feed","actor":"","feed":"at://curator.example.com/app.bsky.feed.generator/startups","query":"","sort":"","identifier":"","limit":30}`},
		{"feed web url with did", `{"mode":"feed","feed":"https://bsky.app/profile/did:plc:abc123/feed/startups/"}`, `{"mode":"feed","actor":"","feed":"at://did:plc:abc123/app.bsky.feed.generator/startups","query":"","sort":"","identifier":"","limit":30}`},
		{"search defaults", `{"mode":"search","query":"  saas pricing ","identifier":" me@example.com "}`, `{"mode":"search","actor":"","feed":"","query":"saas pricing","sort":"top","identifier":"me@example.com","limit":30}`},
		{"search latest", `{"mode":"search","query":"x","sort":"latest","identifier":"me.bsky.social","limit":5}`, `{"mode":"search","actor":"","feed":"","query":"x","sort":"latest","identifier":"me.bsky.social","limit":5}`},
		{"author clears other modes", `{"mode":"author","actor":"alice.bsky.social","feed":"junk","query":"junk","sort":"latest","identifier":"junk"}`, `{"mode":"author","actor":"alice.bsky.social","feed":"","query":"","sort":"","identifier":"","limit":30}`},
		{"feed clears other modes", `{"mode":"feed","feed":"at://did:plc:abc123/app.bsky.feed.generator/x","actor":"bad actor","query":"junk","sort":"nope","identifier":"junk"}`, `{"mode":"feed","actor":"","feed":"at://did:plc:abc123/app.bsky.feed.generator/x","query":"","sort":"","identifier":"","limit":30}`},
		{"search clears other modes", `{"mode":"search","query":"x","identifier":"me","actor":"bad actor","feed":"junk"}`, `{"mode":"search","actor":"","feed":"","query":"x","sort":"top","identifier":"me","limit":30}`},
	}
	for _, tc := range valid {
		got, err := c.Normalize(json.RawMessage(tc.in))
		require.NoError(t, err, tc.name)
		assert.JSONEq(t, tc.want, string(got), tc.name)
	}

	invalid := []struct{ name, in, field string }{
		{"empty", `{}`, "actor"},
		{"author without dot", `{"actor":"alice"}`, "actor"},
		{"author with spaces", `{"actor":"alice smith.com"}`, "actor"},
		{"author bad did method", `{"actor":"did:key:abc"}`, "actor"},
		{"author bad did chars", `{"actor":"did:plc:a/b"}`, "actor"},
		{"unknown mode", `{"mode":"firehose"}`, "mode"},
		{"feed missing", `{"mode":"feed"}`, "feed"},
		{"feed wrong collection", `{"mode":"feed","feed":"at://did:plc:abc123/app.bsky.feed.post/x"}`, "feed"},
		{"feed missing rkey", `{"mode":"feed","feed":"at://did:plc:abc123/app.bsky.feed.generator"}`, "feed"},
		{"feed bad rkey", `{"mode":"feed","feed":"at://did:plc:abc123/app.bsky.feed.generator/a b"}`, "feed"},
		{"feed other host", `{"mode":"feed","feed":"https://example.com/profile/alice.bsky.social/feed/x"}`, "feed"},
		{"feed web url of a profile", `{"mode":"feed","feed":"https://bsky.app/profile/alice.bsky.social"}`, "feed"},
		{"feed http", `{"mode":"feed","feed":"http://bsky.app/profile/alice.bsky.social/feed/x"}`, "feed"},
		{"feed bad authority", `{"mode":"feed","feed":"at://nodot/app.bsky.feed.generator/x"}`, "feed"},
		{"search without query", `{"mode":"search","identifier":"me"}`, "query"},
		{"search blank query", `{"mode":"search","query":"   ","identifier":"me"}`, "query"},
		{"search long query", `{"mode":"search","query":"` + repeat("é", 301) + `","identifier":"me"}`, "query"},
		{"search bad sort", `{"mode":"search","query":"x","sort":"hot","identifier":"me"}`, "sort"},
		{"search without identifier", `{"mode":"search","query":"x"}`, "identifier"},
		{"search long identifier", `{"mode":"search","query":"x","identifier":"` + repeat("a", 255) + `"}`, "identifier"},
		{"limit too high", `{"actor":"alice.bsky.social","limit":101}`, "limit"},
		{"limit negative", `{"actor":"alice.bsky.social","limit":-1}`, "limit"},
		{"not an object", `[1]`, "config"},
	}
	for _, tc := range invalid {
		_, err := c.Normalize(json.RawMessage(tc.in))
		require.Error(t, err, tc.name)
		var ce *ConfigError
		require.ErrorAs(t, err, &ce, tc.name)
		assert.Equal(t, tc.field, ce.Field, tc.name)
	}

	// 300 runes is accepted.
	_, err := c.Normalize(json.RawMessage(`{"mode":"search","query":"` + repeat("é", 300) + `","identifier":"me"}`))
	require.NoError(t, err)
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for range n {
		out = append(out, s...)
	}
	return string(out)
}

func TestBlueskySecret(t *testing.T) {
	t.Parallel()
	c := NewBluesky(http.DefaultClient, "", "", time.Now)
	for _, tc := range []struct {
		name, in string
		want     SecretMode
	}{
		{"author", `{"actor":"alice.bsky.social"}`, SecretNone},
		{"default mode", `{}`, SecretNone},
		{"feed", `{"mode":"feed","feed":"at://did:plc:abc123/app.bsky.feed.generator/x"}`, SecretNone},
		{"search", `{"mode":"search","query":"x","identifier":"me"}`, SecretRequired},
		{"invalid config", `[1]`, SecretNone},
	} {
		assert.Equal(t, tc.want, c.Secret(json.RawMessage(tc.in)), tc.name)
	}
}
