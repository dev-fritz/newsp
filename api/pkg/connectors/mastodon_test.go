package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMastodonHashtagFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/mastodon_tag.json")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/timelines/tag/golang", r.URL.Path)
		assert.Equal(t, "15", r.URL.Query().Get("limit"))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	got, err := NewMastodon(srv.Client(), srv.URL).Fetch(context.Background(),
		json.RawMessage(`{"instance":"social.example","hashtag":"#golang","limit":15}`), "")
	require.NoError(t, err)
	require.Len(t, got, 2, "reblog, private and empty statuses are skipped")

	first := got[0]
	assert.Equal(t, "https://social.example/users/alice/statuses/111001", first.ExternalID)
	assert.Equal(t, "https://social.example/@alice/111001", first.URL)
	assert.Equal(t, "Shipped a tiny #golang tool & it works. Details below.", first.Title, "HTML is stripped")
	assert.Equal(t, first.Title, first.Summary)
	assert.Equal(t, "alice", first.Author)
	assert.Equal(t, time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC), first.PublishedAt)
	assert.Equal(t, 12, *first.Score)
	assert.Equal(t, 2, *first.Comments)
	assert.Equal(t, "social.example", first.Metrics["instance"])
	assert.Equal(t, 3, first.Metrics["reblogs"])
	assert.Equal(t, "en", first.Metrics["language"])
	assert.Equal(t, "https://example.com/post", first.Metrics["link_url"])

	second := got[1]
	assert.Equal(t, "https://other.example/users/bob/statuses/222", second.URL, "url null falls back to uri")
	assert.Equal(t, "Spoiler topic text", second.Title)
	assert.Equal(t, "CW: politics — Spoiler topic text", second.Summary)
	assert.Equal(t, "bob@other.example", second.Author)
	assert.NotContains(t, second.Metrics, "link_url")
}

func TestMastodonAccountFetch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/mastodon_account.json")
	require.NoError(t, err)
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/accounts/lookup":
			assert.Equal(t, "Alice@social.example", r.URL.Query().Get("acct"), "user keeps its case, host is lowercased")
			_, _ = w.Write([]byte(`{"id":"109876","acct":"alice"}`))
		case "/api/v1/accounts/109876/statuses":
			q := r.URL.Query()
			assert.Equal(t, "20", q.Get("limit"))
			assert.Equal(t, "true", q.Get("exclude_replies"))
			assert.Equal(t, "true", q.Get("exclude_reblogs"))
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	got, err := NewMastodon(srv.Client(), srv.URL).Fetch(context.Background(),
		json.RawMessage(`{"instance":"https://Social.Example/","account":"@Alice@Social.Example"}`), "")
	require.NoError(t, err)
	assert.Equal(t, []string{"/api/v1/accounts/lookup", "/api/v1/accounts/109876/statuses"}, calls)
	require.Len(t, got, 1)
	assert.Equal(t, "https://social.example/users/alice/statuses/333001", got[0].ExternalID)
	assert.Equal(t, "Weekly note: pricing experiments for a small SaaS.", got[0].Title)
	assert.Equal(t, 7, *got[0].Score)
	assert.Equal(t, 0, *got[0].Comments)
}

func TestMastodonLongTextTitleIsBounded(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("word ", 100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		b, _ := json.Marshal([]map[string]any{{
			"id": "1", "uri": "https://social.example/s/1", "created_at": "2026-10-25T10:00:00Z",
			"content": "<p>" + long + "</p>", "visibility": "public",
			"account": map[string]string{"acct": "a"},
		}})
		_, _ = w.Write(b)
	}))
	defer srv.Close()
	got, err := NewMastodon(srv.Client(), srv.URL).Fetch(context.Background(),
		json.RawMessage(`{"instance":"social.example","hashtag":"go"}`), "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.LessOrEqual(t, len([]rune(got[0].Title)), maxTitleRunes+1)
	assert.True(t, strings.HasSuffix(got[0].Title, "…"))
	assert.Equal(t, "https://social.example/s/1", got[0].URL, "no url falls back to uri")
}

func TestMastodonLookupNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/accounts/lookup", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := NewMastodon(srv.Client(), srv.URL).Fetch(context.Background(),
		json.RawMessage(`{"instance":"social.example","account":"ghost"}`), "")
	require.Error(t, err)
	assert.False(t, IsConfigError(err))
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "@ghost@social.example")
}

func TestMastodonRejectsUnsafeAccountID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/lookup" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"../../admin"}`))
	}))
	defer srv.Close()
	_, err := NewMastodon(srv.Client(), srv.URL).Fetch(context.Background(),
		json.RawMessage(`{"instance":"social.example","account":"alice"}`), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected account id")
}

func TestMastodonFetchInvalidConfig(t *testing.T) {
	t.Parallel()
	_, err := NewMastodon(http.DefaultClient, "").Fetch(context.Background(), json.RawMessage(`{"instance":"localhost","hashtag":"go"}`), "")
	require.Error(t, err)
	assert.True(t, IsConfigError(err))
}

func TestMastodonNormalize(t *testing.T) {
	t.Parallel()
	c := NewMastodon(http.DefaultClient, "")
	assert.Equal(t, "mastodon", c.Kind())
	assert.Equal(t, SecretNone, c.Secret(nil))

	valid := []struct{ name, in, want string }{
		{"defaults", `{"instance":"mastodon.social","hashtag":"golang"}`,
			`{"instance":"mastodon.social","hashtag":"golang","account":"","limit":20}`},
		{"instance normalized", `{"instance":" https://Mastodon.Social/ ","hashtag":"golang"}`,
			`{"instance":"mastodon.social","hashtag":"golang","account":"","limit":20}`},
		{"hashtag hash prefix and unicode", `{"instance":"mastodon.social","hashtag":"#programação"}`,
			`{"instance":"mastodon.social","hashtag":"programação","account":"","limit":20}`},
		{"local account", `{"instance":"mastodon.social","account":"@Gargron"}`,
			`{"instance":"mastodon.social","hashtag":"","account":"Gargron","limit":20}`},
		{"remote account", `{"instance":"mastodon.social","account":"user_1@Other.Host"}`,
			`{"instance":"mastodon.social","hashtag":"","account":"user_1@other.host","limit":20}`},
		{"limit min", `{"instance":"a.b","hashtag":"x","limit":1}`,
			`{"instance":"a.b","hashtag":"x","account":"","limit":1}`},
		{"limit max", `{"instance":"a-b.example.com","hashtag":"x","limit":40}`,
			`{"instance":"a-b.example.com","hashtag":"x","account":"","limit":40}`},
	}
	for _, tc := range valid {
		got, err := c.Normalize(json.RawMessage(tc.in))
		require.NoError(t, err, tc.name)
		assert.JSONEq(t, tc.want, string(got), tc.name)
		again, err := c.Normalize(got)
		require.NoError(t, err, tc.name)
		assert.JSONEq(t, string(got), string(again), "%s: normalize is idempotent", tc.name)
	}

	invalid := []struct{ name, in, field string }{
		{"no instance", `{"hashtag":"go"}`, "instance"},
		{"localhost", `{"instance":"localhost","hashtag":"go"}`, "instance"},
		{"ipv4", `{"instance":"10.0.0.1","hashtag":"go"}`, "instance"},
		{"ipv6", `{"instance":"::1","hashtag":"go"}`, "instance"},
		{"port", `{"instance":"host.example:8080","hashtag":"go"}`, "instance"},
		{"path", `{"instance":"a/b","hashtag":"go"}`, "instance"},
		{"userinfo", `{"instance":"user@host.example","hashtag":"go"}`, "instance"},
		{"http scheme", `{"instance":"http://host.example","hashtag":"go"}`, "instance"},
		{"leading hyphen", `{"instance":"-a.example","hashtag":"go"}`, "instance"},
		{"empty label", `{"instance":"a..example","hashtag":"go"}`, "instance"},
		{"too long", `{"instance":"` + strings.Repeat("a.", 130) + `com","hashtag":"go"}`, "instance"},
		{"neither", `{"instance":"mastodon.social"}`, "hashtag"},
		{"both", `{"instance":"mastodon.social","hashtag":"go","account":"bob"}`, "hashtag"},
		{"hashtag only hash", `{"instance":"mastodon.social","hashtag":"#"}`, "hashtag"},
		{"hashtag with space", `{"instance":"mastodon.social","hashtag":"go lang"}`, "hashtag"},
		{"hashtag too long", `{"instance":"mastodon.social","hashtag":"` + strings.Repeat("a", 101) + `"}`, "hashtag"},
		{"account bad user", `{"instance":"mastodon.social","account":"bad-user"}`, "account"},
		{"account long user", `{"instance":"mastodon.social","account":"` + strings.Repeat("a", 31) + `"}`, "account"},
		{"account bad domain", `{"instance":"mastodon.social","account":"bob@localhost"}`, "account"},
		{"account two ats", `{"instance":"mastodon.social","account":"bob@a.example@b.example"}`, "account"},
		{"limit zero-negative", `{"instance":"mastodon.social","hashtag":"go","limit":-1}`, "limit"},
		{"limit too high", `{"instance":"mastodon.social","hashtag":"go","limit":41}`, "limit"},
		{"not an object", `[1]`, "config"},
	}
	for _, tc := range invalid {
		_, err := c.Normalize(json.RawMessage(tc.in))
		require.Error(t, err, tc.name)
		require.True(t, IsConfigError(err), tc.name)
		var ce *ConfigError
		require.ErrorAs(t, err, &ce, tc.name)
		assert.Equal(t, tc.field, ce.Field, tc.name)
	}
}
