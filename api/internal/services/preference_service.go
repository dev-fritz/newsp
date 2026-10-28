package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

// Preference limits.
const (
	MaxTopics       = 20
	MaxTopicRunes   = 60
	DefaultTimezone = "America/Sao_Paulo"
)

// PreferenceInput replaces a user's preferences.
type PreferenceInput struct {
	Topics            []string
	OutputLang        string
	Timezone          string
	ReportEnabled     bool
	ReportTime        string // HH:MM
	IdeasPerDay       int
	ReportModelID     *uuid.UUID
	IdeaModelID       *uuid.UUID
	Theme             string
	ExcludedSourceIDs []uuid.UUID
	UILocale          string
}

// PreferenceView is what the profile screen shows.
type PreferenceView struct {
	Preferences models.UserPreferences
	UILocale    string
	// HasTopicsEmbedding tells whether relevance ranking is active for the topics.
	HasTopicsEmbedding bool
}

// PreferenceService manages the caller's own preferences.
type PreferenceService struct {
	tx       TxRunner
	prefs    PreferenceStore
	accounts AccountStore
	models   ModelLookup
	settings SettingsStore
	embedder TextEmbedder
	now      func() time.Time
}

// NewPreferenceService builds a PreferenceService.
func NewPreferenceService(tx TxRunner, prefs PreferenceStore, accounts AccountStore, models ModelLookup,
	settings SettingsStore, embedder TextEmbedder, now func() time.Time,
) *PreferenceService {
	return &PreferenceService{tx: tx, prefs: prefs, accounts: accounts, models: models, settings: settings, embedder: embedder, now: now}
}

// DefaultPreferences are used until a user saves their own.
func DefaultPreferences(userID uuid.UUID, locale string) models.UserPreferences {
	return models.UserPreferences{
		UserID: userID, Topics: []string{}, OutputLang: locale, Timezone: DefaultTimezone,
		ReportEnabled: true, ReportTime: "08:00", IdeasPerDay: 3, Theme: "system", ExcludedSourceIDs: []uuid.UUID{},
	}
}

// initialPreferences are the defaults stored with a new account, scheduled for
// the next default report time.
func initialPreferences(u models.User, now time.Time) (models.UserPreferences, error) {
	p := DefaultPreferences(u.ID, u.Locale)
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return models.UserPreferences{}, fmt.Errorf("default timezone: %w", err)
	}
	hour, minute, _ := parseClock(p.ReportTime)
	next := NextReportAt(now, loc, hour, minute)
	p.NextReportAt = &next
	p.UpdatedAt = now
	return p, nil
}

// Get returns the caller's preferences, or the defaults if never saved.
func (s *PreferenceService) Get(ctx context.Context, actor auth.Identity) (PreferenceView, error) {
	user, err := s.accounts.GetByID(ctx, actor.UserID)
	if err != nil {
		return PreferenceView{}, fmt.Errorf("get preferences: %w", err)
	}
	p, err := s.prefs.Get(ctx, actor.UserID)
	if errors.Is(err, repositories.ErrNotFound) {
		return PreferenceView{Preferences: DefaultPreferences(actor.UserID, user.Locale), UILocale: user.Locale}, nil
	}
	if err != nil {
		return PreferenceView{}, fmt.Errorf("get preferences: %w", err)
	}
	return PreferenceView{Preferences: p, UILocale: user.Locale, HasTopicsEmbedding: p.TopicsEmbeddingModel != nil}, nil
}

// Update validates and replaces the caller's preferences. Topics are embedded
// right away when an embedding model is configured; if that fails, ranking falls
// back to recency until the next reindex.
func (s *PreferenceService) Update(ctx context.Context, actor auth.Identity, in PreferenceInput) (PreferenceView, error) {
	fields := map[string]string{}
	topics, reason := normalizeTopics(in.Topics)
	if reason != "" {
		fields["topics"] = reason
	}
	loc, err := time.LoadLocation(in.Timezone)
	if err != nil || in.Timezone == "" || in.Timezone == "Local" {
		fields["timezone"] = "timezone"
	}
	hour, minute, ok := parseClock(in.ReportTime)
	if !ok {
		fields["report_time"] = "time"
	}
	previous, err := s.prefs.Get(ctx, actor.UserID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return PreferenceView{}, fmt.Errorf("update preferences: %w", err)
	}
	// A model already saved for the field is kept as is: it was checked when it
	// was chosen, and EffectiveModels falls back while it is unusable (BYOK off,
	// disabled). Revalidating it would block saving unrelated fields.
	for _, pick := range []struct {
		field    string
		id, kept *uuid.UUID
	}{
		{"report_model_id", in.ReportModelID, previous.ReportModelID},
		{"idea_model_id", in.IdeaModelID, previous.IdeaModelID},
	} {
		id, field := pick.id, pick.field
		if id == nil || (pick.kept != nil && *pick.kept == *id) {
			continue
		}
		usable, err := s.chatModelUsable(ctx, *id, actor.UserID)
		if err != nil {
			return PreferenceView{}, fmt.Errorf("update preferences: %w", err)
		}
		if !usable {
			fields[field] = "unavailable"
		}
	}
	if len(fields) > 0 {
		return PreferenceView{}, apperrors.Validation("invalid request", fields)
	}

	now := s.now()
	p := models.UserPreferences{
		UserID: actor.UserID, Topics: topics, OutputLang: in.OutputLang, Timezone: in.Timezone,
		ReportEnabled: in.ReportEnabled, ReportTime: fmt.Sprintf("%02d:%02d", hour, minute), IdeasPerDay: in.IdeasPerDay,
		ReportModelID: in.ReportModelID, IdeaModelID: in.IdeaModelID, Theme: in.Theme,
		ExcludedSourceIDs: dedupeIDs(in.ExcludedSourceIDs), UpdatedAt: now,
	}
	if p.ReportEnabled {
		next := NextReportAt(now, loc, hour, minute)
		p.NextReportAt = &next
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.prefs.Save(ctx, p); err != nil {
			if errors.Is(err, repositories.ErrReferenced) {
				return apperrors.Validation("invalid request", map[string]string{"excluded_source_ids": "unknown"})
			}
			return err
		}
		return s.accounts.UpdateLocale(ctx, actor.UserID, in.UILocale, now)
	})
	if err != nil {
		return PreferenceView{}, fmt.Errorf("update preferences: %w", err)
	}

	if len(topics) > 0 && !sameTopics(previous.Topics, topics) {
		s.embedTopics(ctx, actor.UserID, topics)
	}
	return s.Get(ctx, actor)
}

// embedTopics is best effort: a missing or failing embedding model must not
// block saving the profile.
func (s *PreferenceService) embedTopics(ctx context.Context, userID uuid.UUID, topics []string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	vectors, model, err := s.embedder.Embed(ctx, models.PurposeEmbedding, []string{TopicsText(topics)})
	if err != nil || len(vectors) != 1 {
		return
	}
	_ = s.prefs.SetTopicsEmbedding(ctx, userID, vectors[0], model)
}

// TopicsText is the text embedded for a user's interests.
func TopicsText(topics []string) string {
	return "Topics of interest: " + strings.Join(topics, "; ")
}

// EffectiveModels returns the user's report and idea models, falling back to
// the instance defaults. Either may be nil when nothing is configured. A model
// of the user's own provider falls back too while BYOK is switched off or when
// the user disabled the model or its provider.
func (s *PreferenceService) EffectiveModels(ctx context.Context, userID uuid.UUID) (report, idea *uuid.UUID, err error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("effective models: %w", err)
	}
	report, idea = settings.DefaultReportModelID, settings.DefaultIdeaModelID
	p, err := s.prefs.Get(ctx, userID)
	if errors.Is(err, repositories.ErrNotFound) {
		return report, idea, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("effective models: %w", err)
	}
	for _, pick := range []struct{ chosen, effective **uuid.UUID }{{&p.ReportModelID, &report}, {&p.IdeaModelID, &idea}} {
		if *pick.chosen == nil {
			continue
		}
		usable, err := s.usableChoice(ctx, **pick.chosen, userID, settings.BYOKEnabled)
		if err != nil {
			return nil, nil, fmt.Errorf("effective models: %w", err)
		}
		if usable {
			*pick.effective = *pick.chosen
		}
	}
	return report, idea, nil
}

// usableChoice tells whether a saved model choice still applies to userID.
// Instance models are returned as they are (the gateway decides about them).
// User-owned models apply only to their owner, while BYOK is on and while both
// the model and its provider are enabled. A model that no longer exists does
// not apply.
func (s *PreferenceService) usableChoice(ctx context.Context, id, userID uuid.UUID, byok bool) (bool, error) {
	mp, err := s.models.GetModel(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	owner := mp.Provider.UserID
	if owner == nil {
		return true, nil
	}
	return byok && *owner == userID && mp.Model.Enabled && mp.Provider.Enabled, nil
}

func (s *PreferenceService) chatModelUsable(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	mp, err := s.models.GetModel(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !mp.Model.Enabled || !mp.Provider.Enabled || mp.Model.Capability != models.CapabilityChat {
		return false, nil
	}
	if owner := mp.Provider.UserID; owner != nil {
		if *owner != userID {
			return false, nil
		}
		settings, err := s.settings.Get(ctx)
		if err != nil {
			return false, err
		}
		return settings.BYOKEnabled, nil
	}
	return true, nil
}

// NextReportAt is the next moment, strictly after now, when the wall clock in
// loc shows hour:minute. Days are added on the calendar so DST changes keep the
// local time. The result is in UTC.
func NextReportAt(now time.Time, loc *time.Location, hour, minute int) time.Time {
	local := now.In(loc)
	candidate := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !candidate.After(now) {
		candidate = time.Date(local.Year(), local.Month(), local.Day()+1, hour, minute, 0, 0, loc)
	}
	return candidate.UTC()
}

// normalizeTopics trims, collapses spaces, drops empties and duplicates
// (case-insensitively) and enforces the limits.
func normalizeTopics(in []string) ([]string, string) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.Join(strings.Fields(t), " ")
		if t == "" {
			continue
		}
		if utf8.RuneCountInString(t) > MaxTopicRunes {
			return nil, "max"
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	if len(out) > MaxTopics {
		return nil, "max"
	}
	return out, ""
}

func parseClock(s string) (hour, minute int, ok bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, false
	}
	return t.Hour(), t.Minute(), true
}

func sameTopics(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func dedupeIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
