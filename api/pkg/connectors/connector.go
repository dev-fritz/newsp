// Package connectors fetches items from public sources (Hacker News, RSS,
// TabNews, Reddit, YouTube, Lobsters, dev.to, Mastodon, Bluesky, GitHub,
// Product Hunt) and normalizes them.
package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dev-fritz/newsp/api/pkg/safehttp"
)

// Item is a normalized post, story, video or article.
type Item struct {
	ExternalID  string
	URL         string
	Title       string
	Summary     string
	Author      string
	PublishedAt time.Time
	Score       *int
	Comments    *int
	Metrics     map[string]any
}

// Doer sends HTTP requests. Production code passes a safehttp client.
type Doer = safehttp.Doer

// SecretMode tells whether a source takes a credential (API token, app
// password). The credential is stored encrypted, apart from the config.
type SecretMode int

// Secret modes.
const (
	SecretNone SecretMode = iota
	SecretOptional
	SecretRequired
)

// Connector reads one kind of source.
type Connector interface {
	// Kind is the value stored in sources.kind.
	Kind() string
	// Normalize validates a source config and returns it with defaults applied.
	Normalize(config json.RawMessage) (json.RawMessage, error)
	// Secret tells whether a normalized config takes a credential.
	Secret(config json.RawMessage) SecretMode
	// Fetch returns the latest items for a normalized config. secret is empty
	// when the source has none; it must never appear in URLs or errors.
	Fetch(ctx context.Context, config json.RawMessage, secret string) ([]Item, error)
}

// ConfigError is an invalid source configuration (shown to the admin).
type ConfigError struct {
	Field  string
	Reason string
}

func (e *ConfigError) Error() string { return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason) }

// IsConfigError reports whether err is a ConfigError.
func IsConfigError(err error) bool {
	var ce *ConfigError
	return errors.As(err, &ce)
}

// Registry looks connectors up by kind.
type Registry struct {
	byKind map[string]Connector
}

// NewRegistry builds a Registry from connectors.
func NewRegistry(cs ...Connector) *Registry {
	r := &Registry{byKind: make(map[string]Connector, len(cs))}
	for _, c := range cs {
		r.byKind[c.Kind()] = c
	}
	return r
}

// Get returns the connector for kind.
func (r *Registry) Get(kind string) (Connector, bool) {
	c, ok := r.byKind[kind]
	return c, ok
}

// Kinds lists registered kinds, sorted.
func (r *Registry) Kinds() []string {
	kinds := make([]string, 0, len(r.byKind))
	for k := range r.byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

// decodeConfig strictly decodes a JSON config into dst.
func decodeConfig(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return &ConfigError{Field: "config", Reason: "must be a JSON object with the documented fields"}
	}
	return nil
}

func encodeConfig(v any) (json.RawMessage, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return raw, nil
}

func intPtr(v int) *int { return &v }
