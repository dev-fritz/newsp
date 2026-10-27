package connectors

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dev-fritz/newsp/api/pkg/safehttp"
)

const (
	blueskyAppViewURL = "https://public.api.bsky.app"
	blueskyPDSURL     = "https://bsky.social"

	// blueskySessionTTL is how long a createSession token is reused. createSession
	// is rate limited (about 30 per 5 minutes and 300 per day per account) and
	// access tokens live for a couple of hours.
	blueskySessionTTL = 90 * time.Minute

	blueskyFeedCollection = "app.bsky.feed.generator"
	blueskyMaxQueryRunes  = 300
	blueskyMaxIdentifier  = 254
)

var (
	blueskyHandlePattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`)
	blueskyDIDPattern    = regexp.MustCompile(`^did:(plc|web):[a-zA-Z0-9._:%-]+$`)
	blueskyRkeyPattern   = regexp.MustCompile(`^[A-Za-z0-9._~-]{1,512}$`)
)

// BlueskyConfig selects what to read from Bluesky.
type BlueskyConfig struct {
	Mode       string `json:"mode"`       // author (default) | feed | search
	Actor      string `json:"actor"`      // author: handle or DID
	Feed       string `json:"feed"`       // feed: at:// URI or bsky.app feed URL
	Query      string `json:"query"`      // search: query text
	Sort       string `json:"sort"`       // search: top (default) | latest
	Identifier string `json:"identifier"` // search: handle or email that logs in
	Limit      int    `json:"limit"`      // 1..100
}

// Bluesky reads posts from an account, a custom feed or a search. Author and
// feed reads are public (AppView); search needs a login because the public
// AppView refuses searchPosts, so it uses the app password of the configured
// account against the account's PDS.
type Bluesky struct {
	client  Doer
	appView string
	pds     string
	now     func() time.Time

	mu       sync.Mutex
	sessions map[string]blueskySession
}

type blueskySession struct {
	accessJWT string
	expires   time.Time
}

// NewBluesky builds the connector. appViewURL and pdsURL are empty in production.
func NewBluesky(client Doer, appViewURL, pdsURL string, now func() time.Time) *Bluesky {
	if appViewURL == "" {
		appViewURL = blueskyAppViewURL
	}
	if pdsURL == "" {
		pdsURL = blueskyPDSURL
	}
	return &Bluesky{
		client:   client,
		appView:  strings.TrimRight(appViewURL, "/"),
		pds:      strings.TrimRight(pdsURL, "/"),
		now:      now,
		sessions: make(map[string]blueskySession),
	}
}

// Kind implements Connector.
func (*Bluesky) Kind() string { return "bluesky" }

// Secret implements Connector: search logs in with an app password.
func (b *Bluesky) Secret(raw json.RawMessage) SecretMode {
	norm, err := b.Normalize(raw)
	if err != nil {
		return SecretNone
	}
	var c BlueskyConfig
	if err := json.Unmarshal(norm, &c); err != nil {
		return SecretNone
	}
	if c.Mode == "search" {
		return SecretRequired
	}
	return SecretNone
}

// Normalize implements Connector.
func (*Bluesky) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var in BlueskyConfig
	if err := decodeConfig(raw, &in); err != nil {
		return nil, err
	}
	out := BlueskyConfig{Mode: in.Mode, Limit: in.Limit}
	if out.Mode == "" {
		out.Mode = "author"
	}
	switch out.Mode {
	case "author":
		actor, ok := normalizeBlueskyActor(in.Actor)
		if !ok {
			return nil, &ConfigError{Field: "actor", Reason: "must be a handle such as alice.bsky.social or a DID (did:plc:… or did:web:…)"}
		}
		out.Actor = actor
	case "feed":
		feed, ok := normalizeBlueskyFeed(in.Feed)
		if !ok {
			return nil, &ConfigError{Field: "feed", Reason: "must be an at://…/app.bsky.feed.generator/… URI or a https://bsky.app/profile/…/feed/… URL"}
		}
		out.Feed = feed
	case "search":
		q := strings.TrimSpace(in.Query)
		if n := utf8.RuneCountInString(q); n < 1 || n > blueskyMaxQueryRunes {
			return nil, &ConfigError{Field: "query", Reason: "must have 1 to 300 characters"}
		}
		out.Query = q
		out.Sort = in.Sort
		if out.Sort == "" {
			out.Sort = "top"
		}
		if out.Sort != "top" && out.Sort != "latest" {
			return nil, &ConfigError{Field: "sort", Reason: "must be top or latest"}
		}
		id := strings.TrimSpace(in.Identifier)
		if id == "" || len(id) > blueskyMaxIdentifier {
			return nil, &ConfigError{Field: "identifier", Reason: "must be the handle or email of the account that logs in (up to 254 characters)"}
		}
		out.Identifier = id
	default:
		return nil, &ConfigError{Field: "mode", Reason: "must be author, feed or search"}
	}
	if out.Limit == 0 {
		out.Limit = 30
	}
	if out.Limit < 1 || out.Limit > 100 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 100"}
	}
	return encodeConfig(out)
}

// normalizeBlueskyHandle lowercases and validates a handle (without "@").
func normalizeBlueskyHandle(s string) (string, bool) {
	s = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), "@"))
	if len(s) > 253 || !blueskyHandlePattern.MatchString(s) {
		return "", false
	}
	return s, true
}

// normalizeBlueskyAuthority accepts a handle or a DID.
func normalizeBlueskyAuthority(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if blueskyDIDPattern.MatchString(s) {
		return s, true
	}
	return normalizeBlueskyHandle(s)
}

func normalizeBlueskyActor(s string) (string, bool) { return normalizeBlueskyAuthority(s) }

// normalizeBlueskyFeed returns the at:// URI of a feed given as at:// or as a
// bsky.app web URL.
func normalizeBlueskyFeed(s string) (string, bool) {
	s = strings.TrimSpace(s)
	var authority, rkey string
	switch {
	case strings.HasPrefix(s, "at://"):
		parts := strings.Split(strings.TrimPrefix(s, "at://"), "/")
		if len(parts) != 3 || parts[1] != blueskyFeedCollection {
			return "", false
		}
		authority, rkey = parts[0], parts[2]
	case strings.HasPrefix(s, "https://"):
		u, err := url.Parse(s)
		if err != nil || u.Host != "bsky.app" {
			return "", false
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "profile" || parts[2] != "feed" {
			return "", false
		}
		authority, rkey = parts[1], parts[3]
	default:
		return "", false
	}
	authority, ok := normalizeBlueskyAuthority(authority)
	if !ok || !blueskyRkeyPattern.MatchString(rkey) {
		return "", false
	}
	return "at://" + authority + "/" + blueskyFeedCollection + "/" + rkey, true
}

type blueskyAuthor struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type blueskyPost struct {
	URI    string        `json:"uri"`
	Author blueskyAuthor `json:"author"`
	Record struct {
		Text      string `json:"text"`
		CreatedAt string `json:"createdAt"`
	} `json:"record"`
	IndexedAt   string `json:"indexedAt"`
	LikeCount   int    `json:"likeCount"`
	ReplyCount  int    `json:"replyCount"`
	RepostCount int    `json:"repostCount"`
	QuoteCount  int    `json:"quoteCount"`
	Embed       *struct {
		Type     string `json:"$type"`
		External struct {
			URI   string `json:"uri"`
			Title string `json:"title"`
		} `json:"external"`
	} `json:"embed"`
}

type blueskyFeedResponse struct {
	Feed []struct {
		Post   blueskyPost     `json:"post"`
		Reason json.RawMessage `json:"reason"`
	} `json:"feed"`
}

type blueskySearchResponse struct {
	Posts []blueskyPost `json:"posts"`
}

// Fetch implements Connector.
func (b *Bluesky) Fetch(ctx context.Context, raw json.RawMessage, secret string) ([]Item, error) {
	norm, err := b.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c BlueskyConfig
	_ = json.Unmarshal(norm, &c)

	var posts []blueskyPost
	switch c.Mode {
	case "feed":
		posts, err = b.fetchFeed(ctx, c)
	case "search":
		posts, err = b.fetchSearch(ctx, c, secret)
	default:
		posts, err = b.fetchAuthor(ctx, c)
	}
	if err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(posts))
	for _, p := range posts {
		if it, ok := b.item(p); ok {
			items = append(items, it)
		}
	}
	return items, nil
}

func (b *Bluesky) fetchAuthor(ctx context.Context, c BlueskyConfig) ([]blueskyPost, error) {
	q := url.Values{}
	q.Set("actor", c.Actor)
	q.Set("limit", strconv.Itoa(c.Limit))
	q.Set("filter", "posts_no_replies")
	var res blueskyFeedResponse
	if err := getJSON(ctx, b.client, b.appView+"/xrpc/app.bsky.feed.getAuthorFeed?"+q.Encode(), nil, &res); err != nil {
		return nil, fmt.Errorf("bluesky author %s: %w", c.Actor, err)
	}
	return feedPosts(res), nil
}

func (b *Bluesky) fetchFeed(ctx context.Context, c BlueskyConfig) ([]blueskyPost, error) {
	feed := c.Feed
	authority := strings.SplitN(strings.TrimPrefix(feed, "at://"), "/", 2)[0]
	if !strings.HasPrefix(authority, "did:") {
		hq := url.Values{}
		hq.Set("handle", authority)
		var resolved struct {
			DID string `json:"did"`
		}
		if err := getJSON(ctx, b.client, b.appView+"/xrpc/com.atproto.identity.resolveHandle?"+hq.Encode(), nil, &resolved); err != nil {
			return nil, fmt.Errorf("bluesky resolve handle %s: %w", authority, err)
		}
		if !blueskyDIDPattern.MatchString(resolved.DID) {
			return nil, fmt.Errorf("bluesky resolve handle %s: no DID in the response", authority)
		}
		feed = strings.Replace(feed, "at://"+authority, "at://"+resolved.DID, 1)
	}
	q := url.Values{}
	q.Set("feed", feed)
	q.Set("limit", strconv.Itoa(c.Limit))
	var res blueskyFeedResponse
	if err := getJSON(ctx, b.client, b.appView+"/xrpc/app.bsky.feed.getFeed?"+q.Encode(), nil, &res); err != nil {
		return nil, fmt.Errorf("bluesky feed %s: %w", c.Feed, err)
	}
	return feedPosts(res), nil
}

// feedPosts drops reposts (entries with a reason).
func feedPosts(res blueskyFeedResponse) []blueskyPost {
	posts := make([]blueskyPost, 0, len(res.Feed))
	for _, e := range res.Feed {
		if len(e.Reason) > 0 && string(e.Reason) != "null" {
			continue
		}
		posts = append(posts, e.Post)
	}
	return posts
}

func (b *Bluesky) fetchSearch(ctx context.Context, c BlueskyConfig, secret string) ([]blueskyPost, error) {
	if secret == "" {
		return nil, fmt.Errorf("bluesky search needs the app password of %s", c.Identifier)
	}
	q := url.Values{}
	q.Set("q", c.Query)
	q.Set("limit", strconv.Itoa(c.Limit))
	q.Set("sort", c.Sort)
	endpoint := b.pds + "/xrpc/app.bsky.feed.searchPosts?" + q.Encode()

	for attempt := 0; ; attempt++ {
		jwt, err := b.session(ctx, c.Identifier, secret)
		if err != nil {
			return nil, err
		}
		var res blueskySearchResponse
		header := http.Header{"Authorization": {"Bearer " + jwt}}
		err = getJSON(ctx, b.client, endpoint, header, &res)
		if err == nil {
			return res.Posts, nil
		}
		var status *safehttp.StatusError
		if attempt == 0 && errors.As(err, &status) && status.Code == http.StatusUnauthorized {
			b.dropSession(c.Identifier, secret)
			continue
		}
		return nil, fmt.Errorf("bluesky search: %w", err)
	}
}

// sessionKey ties a cached session to the account and to the password used, so
// a changed password never gets a stale session.
func sessionKey(identifier, secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return identifier + "\x00" + hex.EncodeToString(sum[:])
}

func (b *Bluesky) dropSession(identifier, secret string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, sessionKey(identifier, secret))
}

// session returns a cached access token or logs in. The password only travels
// in the createSession body.
func (b *Bluesky) session(ctx context.Context, identifier, secret string) (string, error) {
	key := sessionKey(identifier, secret)
	b.mu.Lock()
	s, ok := b.sessions[key]
	b.mu.Unlock()
	if ok && b.now().Before(s.expires) {
		return s.accessJWT, nil
	}

	payload := map[string]string{"identifier": identifier, "password": secret}
	var res struct {
		AccessJWT string `json:"accessJwt"`
	}
	if err := postJSON(ctx, b.client, b.pds+"/xrpc/com.atproto.server.createSession", nil, payload, &res); err != nil {
		var status *safehttp.StatusError
		if errors.As(err, &status) && status.Code == http.StatusUnauthorized {
			return "", fmt.Errorf("bluesky login failed for %s: check the app password", identifier)
		}
		return "", fmt.Errorf("bluesky login for %s: %w", identifier, err)
	}
	if res.AccessJWT == "" {
		return "", fmt.Errorf("bluesky login for %s: no access token in the response", identifier)
	}

	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()
	for k, v := range b.sessions { // drop expired entries so the map stays small
		if !now.Before(v.expires) {
			delete(b.sessions, k)
		}
	}
	b.sessions[key] = blueskySession{accessJWT: res.AccessJWT, expires: now.Add(blueskySessionTTL)}
	return res.AccessJWT, nil
}

// item maps a post; ok is false for posts that are skipped.
func (b *Bluesky) item(p blueskyPost) (Item, bool) {
	text := strings.TrimSpace(p.Record.Text)
	if text == "" || p.URI == "" {
		return Item{}, false
	}
	rkey := p.URI[strings.LastIndexByte(p.URI, '/')+1:]
	if rkey == "" {
		return Item{}, false
	}
	profile := p.Author.Handle
	if profile == "" {
		profile = p.Author.DID
	}
	if profile == "" {
		return Item{}, false
	}

	published := b.now().UTC()
	for _, s := range []string{p.Record.CreatedAt, p.IndexedAt} {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			published = t.UTC()
			break
		}
	}

	summary := summarize(text)
	metrics := map[string]any{"reposts": p.RepostCount, "quotes": p.QuoteCount}
	if p.Embed != nil && p.Embed.Type == "app.bsky.embed.external#view" {
		if title := strings.TrimSpace(p.Embed.External.Title); title != "" {
			summary += " — " + title
		}
		if link := p.Embed.External.URI; strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "http://") {
			metrics["link_url"] = link
		}
	}

	return Item{
		ExternalID:  p.URI,
		URL:         "https://bsky.app/profile/" + profile + "/post/" + rkey,
		Title:       titleFromText(text),
		Summary:     summary,
		Author:      p.Author.Handle,
		PublishedAt: published,
		Score:       intPtr(p.LikeCount),
		Comments:    intPtr(p.ReplyCount),
		Metrics:     metrics,
	}, true
}
