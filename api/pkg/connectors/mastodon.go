package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	mastodonHashtagPattern = regexp.MustCompile(`^[\p{L}\p{N}_]{1,100}$`)
	mastodonUserPattern    = regexp.MustCompile(`^[A-Za-z0-9_]{1,30}$`)
	mastodonIDPattern      = regexp.MustCompile(`^[A-Za-z0-9]{1,40}$`)
	hostLabelPattern       = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

// MastodonConfig selects a hashtag or account timeline on one instance.
type MastodonConfig struct {
	Instance string `json:"instance"` // DNS hostname, e.g. mastodon.social
	Hashtag  string `json:"hashtag"`  // without "#"
	Account  string `json:"account"`  // "user" or "user@other.host", without the leading "@"
	Limit    int    `json:"limit"`    // 1..40
}

// Mastodon reads public timelines through the Mastodon REST API, which needs
// no token. The instance host is admin-provided: the production client (safehttp)
// refuses private and loopback addresses, which is the SSRF defense.
type Mastodon struct {
	client  Doer
	baseURL string
}

// NewMastodon builds the connector. baseURL is empty in production, when
// requests go to "https://" + instance; tests pass a server URL that replaces it.
func NewMastodon(client Doer, baseURL string) *Mastodon {
	return &Mastodon{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

// Secret implements Connector: public timelines need no credential.
func (*Mastodon) Secret(json.RawMessage) SecretMode { return SecretNone }

// Kind implements Connector.
func (m *Mastodon) Kind() string { return "mastodon" }

// validHostname reports whether h is a plain lowercase DNS hostname with at
// least one dot: no port, path, userinfo or IP literal.
func validHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 || !strings.Contains(h, ".") || net.ParseIP(h) != nil {
		return false
	}
	labels := strings.Split(h, ".")
	for _, l := range labels {
		if !hostLabelPattern.MatchString(l) {
			return false
		}
	}
	// An all-numeric last label would make this a (legacy) IPv4 form.
	last := labels[len(labels)-1]
	return strings.Trim(last, "0123456789") != ""
}

// Normalize implements Connector.
func (m *Mastodon) Normalize(raw json.RawMessage) (json.RawMessage, error) {
	var c MastodonConfig
	if err := decodeConfig(raw, &c); err != nil {
		return nil, err
	}

	inst := strings.ToLower(strings.TrimSpace(c.Instance))
	inst = strings.TrimPrefix(inst, "https://")
	inst = strings.TrimSuffix(inst, "/")
	if !validHostname(inst) {
		return nil, &ConfigError{Field: "instance", Reason: "must be an instance hostname such as mastodon.social (no port, path or IP address)"}
	}
	c.Instance = inst

	tag := strings.TrimPrefix(strings.TrimSpace(c.Hashtag), "#")
	acct := strings.TrimPrefix(strings.TrimSpace(c.Account), "@")
	if (tag == "") == (acct == "") {
		return nil, &ConfigError{Field: "hashtag", Reason: "set either hashtag or account"}
	}
	if tag != "" && !mastodonHashtagPattern.MatchString(tag) {
		return nil, &ConfigError{Field: "hashtag", Reason: "must be a hashtag such as golang or #golang (1-100 letters, digits or underscores)"}
	}
	if acct != "" {
		user, domain, hasDomain := strings.Cut(acct, "@")
		domain = strings.ToLower(domain)
		if !mastodonUserPattern.MatchString(user) || (hasDomain && !validHostname(domain)) {
			return nil, &ConfigError{Field: "account", Reason: "must be user or user@host (user: 1-30 letters, digits or underscores)"}
		}
		acct = user
		if hasDomain {
			acct = user + "@" + domain
		}
	}
	c.Hashtag, c.Account = tag, acct

	if c.Limit == 0 {
		c.Limit = 20
	}
	if c.Limit < 1 || c.Limit > 40 {
		return nil, &ConfigError{Field: "limit", Reason: "must be between 1 and 40"}
	}
	return encodeConfig(c)
}

type mastodonStatus struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	URL         *string   `json:"url"`
	URI         string    `json:"uri"`
	Content     string    `json:"content"`
	SpoilerText string    `json:"spoiler_text"`
	Visibility  string    `json:"visibility"`
	Language    *string   `json:"language"`
	Reblog      *struct{} `json:"reblog"`
	Account     struct {
		Acct string `json:"acct"`
	} `json:"account"`
	FavouritesCount int `json:"favourites_count"`
	ReblogsCount    int `json:"reblogs_count"`
	RepliesCount    int `json:"replies_count"`
	Card            *struct {
		URL string `json:"url"`
	} `json:"card"`
}

// Fetch implements Connector.
func (m *Mastodon) Fetch(ctx context.Context, raw json.RawMessage, _ string) ([]Item, error) {
	norm, err := m.Normalize(raw)
	if err != nil {
		return nil, err
	}
	var c MastodonConfig
	_ = json.Unmarshal(norm, &c)

	base := m.baseURL
	if base == "" {
		base = "https://" + c.Instance
	}

	var (
		label    string
		endpoint string
	)
	if c.Hashtag != "" {
		label = "#" + c.Hashtag + "@" + c.Instance
		endpoint = fmt.Sprintf("%s/api/v1/timelines/tag/%s?limit=%d", base, url.PathEscape(c.Hashtag), c.Limit)
	} else {
		label = "@" + c.Account + "@" + c.Instance
		var acc struct {
			ID string `json:"id"`
		}
		lookup := base + "/api/v1/accounts/lookup?acct=" + url.QueryEscape(c.Account)
		if err := getJSON(ctx, m.client, lookup, nil, &acc); err != nil {
			return nil, fmt.Errorf("mastodon %s lookup: %w", label, err)
		}
		if !mastodonIDPattern.MatchString(acc.ID) {
			return nil, fmt.Errorf("mastodon %s lookup: unexpected account id", label)
		}
		endpoint = fmt.Sprintf("%s/api/v1/accounts/%s/statuses?limit=%d&exclude_replies=true&exclude_reblogs=true", base, acc.ID, c.Limit)
	}

	var statuses []mastodonStatus
	if err := getJSON(ctx, m.client, endpoint, nil, &statuses); err != nil {
		return nil, fmt.Errorf("mastodon %s: %w", label, err)
	}

	items := make([]Item, 0, len(statuses))
	for _, s := range statuses {
		if s.Reblog != nil || (s.Visibility != "public" && s.Visibility != "unlisted") {
			continue
		}
		plain := StripHTML(s.Content)
		externalID := s.URI
		if externalID == "" {
			externalID = s.ID
		}
		if plain == "" || externalID == "" {
			continue
		}
		link := s.URI
		if s.URL != nil && *s.URL != "" {
			link = *s.URL
		}
		summary := summarize(s.Content)
		if cw := strings.TrimSpace(s.SpoilerText); cw != "" {
			summary = "CW: " + cw + " — " + summary
		}
		metrics := map[string]any{"instance": c.Instance, "reblogs": s.ReblogsCount}
		if s.Language != nil {
			metrics["language"] = *s.Language
		}
		if s.Card != nil && (strings.HasPrefix(s.Card.URL, "https://") || strings.HasPrefix(s.Card.URL, "http://")) {
			metrics["link_url"] = s.Card.URL
		}
		items = append(items, Item{
			ExternalID:  externalID,
			URL:         link,
			Title:       titleFromText(plain),
			Summary:     summary,
			Author:      s.Account.Acct,
			PublishedAt: s.CreatedAt.UTC(),
			Score:       intPtr(s.FavouritesCount),
			Comments:    intPtr(s.RepliesCount),
			Metrics:     metrics,
		})
	}
	return items, nil
}
