package services

//go:generate go tool mockgen -source=report_service.go -destination=../../tests/mocks/report_mocks.go -package=mocks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/ai"
)

// Job kinds of the generation pipelines.
const (
	JobGenerateReport = "report.generate"
	JobGenerateIdeas  = "ideas.generate"
)

// Report error codes stored on failed reports; the web app translates them.
const (
	ReportErrNoModel   = "REPORT_MODEL_NOT_SET"
	ReportErrNoItems   = "NO_ITEMS"
	ReportErrRefused   = "AI_REFUSED"
	ReportErrInvalid   = "AI_INVALID_OUTPUT"
	ReportErrAIFailure = "AI_FAILURE"
)

// errEmailNotVerified blocks manual generations until the account confirms its
// e-mail, so unverified signups cannot spend AI budget.
var errEmailNotVerified = apperrors.Forbidden("EMAIL_NOT_VERIFIED", "confirm your e-mail to generate reports and ideas")

// requireVerifiedEmail returns errEmailNotVerified when the user has not
// confirmed their e-mail address.
func requireVerifiedEmail(ctx context.Context, prefs PreferenceStore, userID uuid.UUID) error {
	verified, err := prefs.EmailVerified(ctx, userID)
	if err != nil {
		return fmt.Errorf("check e-mail verification: %w", err)
	}
	if !verified {
		return errEmailNotVerified
	}
	return nil
}

// Report pipeline tuning.
const (
	reportCandidates      = 40
	reportPerSource       = 12
	reportExtractTop      = 15
	extractConcurrency    = 4
	extractTimeout        = 20 * time.Second
	scheduledReportWindow = 24 * time.Hour
)

// ReportStore persists reports.
type ReportStore interface {
	Create(ctx context.Context, rep models.Report) error
	Get(ctx context.Context, id uuid.UUID) (models.Report, error)
	GetForUser(ctx context.Context, id, userID uuid.UUID) (models.Report, error)
	ListForUser(ctx context.Context, userID uuid.UUID, kind string, limit, offset int) ([]models.Report, int, error)
	DeleteForUser(ctx context.Context, id, userID uuid.UUID) error
	HasActive(ctx context.Context, userID uuid.UUID) (bool, error)
	MarkRunning(ctx context.Context, id uuid.UUID) error
	Complete(ctx context.Context, rep models.Report, citations []models.ReportCitation) error
	Fail(ctx context.Context, id uuid.UUID, message string, at time.Time) error
	FailOrphaned(ctx context.Context, message string, at time.Time) (int64, error)
	PreviousSummary(ctx context.Context, userID uuid.UUID, before time.Time) (string, string, error)
	Citations(ctx context.Context, reportID uuid.UUID) ([]repositories.CitedItem, error)
}

// CandidateStore ranks items and caches article text.
type CandidateStore interface {
	Rank(ctx context.Context, q repositories.CandidateQuery) ([]repositories.Candidate, int, error)
	Cited(ctx context.Context, reportID uuid.UUID) ([]repositories.Candidate, error)
	ByIDs(ctx context.Context, ids []uuid.UUID) ([]repositories.Candidate, error)
	SetContent(ctx context.Context, itemID uuid.UUID, text, extractErr *string, at time.Time) error
}

// TopicsReader reads the stored topics embedding of a user.
type TopicsReader interface {
	TopicsEmbedding(ctx context.Context, userID uuid.UUID) ([]float32, string, error)
}

// GenerationAI is what the pipelines need from the AI gateway.
type GenerationAI interface {
	GenerateJSON(ctx context.Context, modelID uuid.UUID, purpose models.UsagePurpose, userID *uuid.UUID, req ai.ChatRequest, dst any, validate func() error) (ResolvedModel, error)
	Embed(ctx context.Context, purpose models.UsagePurpose, texts []string) ([][]float32, string, error)
}

// ArticleExtractor downloads an article and returns its main text.
type ArticleExtractor interface {
	Text(ctx context.Context, pageURL string) (string, error)
}

// UserModels resolves the models a user's generations use.
type UserModels interface {
	EffectiveModels(ctx context.Context, userID uuid.UUID) (report, idea *uuid.UUID, err error)
}

// ReportRequest asks for a report now.
type ReportRequest struct {
	WindowHours int
	Focus       string
}

// ReportDetail is a report with its cited items.
type ReportDetail struct {
	Report    models.Report
	Citations []repositories.CitedItem
}

// ReportService creates, generates and serves market reports.
type ReportService struct {
	tx         TxRunner
	reports    ReportStore
	candidates CandidateStore
	prefs      PreferenceStore
	topics     TopicsReader
	models     UserModels
	ai         GenerationAI
	extractor  ArticleExtractor
	queue      Enqueuer
	now        func() time.Time
}

// NewReportService builds a ReportService.
func NewReportService(tx TxRunner, reports ReportStore, candidates CandidateStore, prefs PreferenceStore, topics TopicsReader,
	userModels UserModels, gen GenerationAI, extractor ArticleExtractor, queue Enqueuer, now func() time.Time,
) *ReportService {
	return &ReportService{
		tx: tx, reports: reports, candidates: candidates, prefs: prefs, topics: topics, models: userModels,
		ai: gen, extractor: extractor, queue: queue, now: now,
	}
}

// Request creates a manual report and queues its generation. A user has at
// most one report in progress.
func (s *ReportService) Request(ctx context.Context, actor auth.Identity, in ReportRequest) (models.Report, uuid.UUID, error) {
	if in.WindowHours < 1 || in.WindowHours > 24*30 {
		return models.Report{}, uuid.Nil, apperrors.Validation("invalid request", map[string]string{"window_hours": "max"})
	}
	if err := requireVerifiedEmail(ctx, s.prefs, actor.UserID); err != nil {
		return models.Report{}, uuid.Nil, err
	}
	lang, err := s.outputLang(ctx, actor.UserID)
	if err != nil {
		return models.Report{}, uuid.Nil, err
	}
	now := s.now()
	rep := models.Report{
		ID: uuid.New(), UserID: actor.UserID, Kind: models.ReportManual, Status: models.ReportPending,
		WindowStart: now.Add(-time.Duration(in.WindowHours) * time.Hour), WindowEnd: now, OutputLang: lang, CreatedAt: now,
	}
	if focus := strings.TrimSpace(in.Focus); focus != "" {
		rep.Focus = &focus
	}

	var jobID uuid.UUID
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		active, err := s.reports.HasActive(ctx, actor.UserID)
		if err != nil {
			return err
		}
		if active {
			return apperrors.Conflict("REPORT_IN_PROGRESS", "a report is already being generated")
		}
		if err := s.reports.Create(ctx, rep); err != nil {
			return err
		}
		jobID, err = s.enqueue(ctx, rep)
		return err
	})
	if err != nil {
		return models.Report{}, uuid.Nil, fmt.Errorf("request report: %w", err)
	}
	return rep, jobID, nil
}

func (s *ReportService) enqueue(ctx context.Context, rep models.Report) (uuid.UUID, error) {
	userID := rep.UserID
	return s.queue.Enqueue(ctx, JobGenerateReport, map[string]string{"report_id": rep.ID.String()},
		workers.EnqueueOptions{UserID: &userID, DedupeKey: "report:" + rep.ID.String(), MaxAttempts: 3})
}

func (s *ReportService) outputLang(ctx context.Context, userID uuid.UUID) (string, error) {
	p, err := s.prefs.Get(ctx, userID)
	if errors.Is(err, repositories.ErrNotFound) {
		return "pt-BR", nil
	}
	if err != nil {
		return "", err
	}
	return p.OutputLang, nil
}

// List returns a page of the caller's reports.
func (s *ReportService) List(ctx context.Context, actor auth.Identity, kind string, limit, offset int) ([]models.Report, int, error) {
	reports, total, err := s.reports.ListForUser(ctx, actor.UserID, kind, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	return reports, total, nil
}

// Get returns one of the caller's reports with its citations.
func (s *ReportService) Get(ctx context.Context, actor auth.Identity, id uuid.UUID) (ReportDetail, error) {
	rep, err := s.reports.GetForUser(ctx, id, actor.UserID)
	if errors.Is(err, repositories.ErrNotFound) {
		return ReportDetail{}, apperrors.NotFound("REPORT_NOT_FOUND", "report not found")
	}
	if err != nil {
		return ReportDetail{}, fmt.Errorf("get report: %w", err)
	}
	citations, err := s.reports.Citations(ctx, rep.ID)
	if err != nil {
		return ReportDetail{}, fmt.Errorf("get report: %w", err)
	}
	return ReportDetail{Report: rep, Citations: citations}, nil
}

// Delete permanently removes one of the caller's finished reports. Reports that
// are still pending or running cannot be deleted. Collected items stay.
func (s *ReportService) Delete(ctx context.Context, actor auth.Identity, id uuid.UUID) error {
	inProgress := apperrors.Conflict("REPORT_IN_PROGRESS", "the report is still being generated; wait for it to finish")
	rep, err := s.reports.GetForUser(ctx, id, actor.UserID)
	if errors.Is(err, repositories.ErrNotFound) {
		return apperrors.NotFound("REPORT_NOT_FOUND", "report not found")
	}
	if err != nil {
		return fmt.Errorf("delete report: %w", err)
	}
	if rep.Status == models.ReportPending || rep.Status == models.ReportRunning {
		return inProgress
	}
	err = s.reports.DeleteForUser(ctx, id, actor.UserID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, repositories.ErrNotFound) {
		return fmt.Errorf("delete report: %w", err)
	}
	// Nothing was deleted: the report vanished or went back to in progress meanwhile.
	_, err = s.reports.GetForUser(ctx, id, actor.UserID)
	if errors.Is(err, repositories.ErrNotFound) {
		return apperrors.NotFound("REPORT_NOT_FOUND", "report not found")
	}
	if err != nil {
		return fmt.Errorf("delete report: %w", err)
	}
	return inProgress
}

// EnqueueDue creates the scheduled reports that are due and reschedules each
// user to the next day. It runs on the worker's scheduler.
func (s *ReportService) EnqueueDue(ctx context.Context, now time.Time) error {
	users, err := s.prefs.DueReportUsers(ctx, now, 100)
	if err != nil {
		return err
	}
	for _, userID := range users {
		if err := s.scheduleUser(ctx, userID, now); err != nil {
			return fmt.Errorf("schedule report for %s: %w", userID, err)
		}
	}
	return nil
}

func (s *ReportService) scheduleUser(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.prefs.Get(ctx, userID)
		if err != nil {
			return err
		}
		loc, err := time.LoadLocation(p.Timezone)
		if err != nil {
			loc = time.UTC
		}
		hour, minute, ok := parseClock(p.ReportTime)
		if !ok {
			hour, minute = 8, 0
		}
		next := NextReportAt(now, loc, hour, minute)
		if err := s.prefs.SetNextReportAt(ctx, userID, &next); err != nil {
			return err
		}
		active, err := s.reports.HasActive(ctx, userID)
		if err != nil || active {
			return err // a report still running covers this slot
		}
		rep := models.Report{
			ID: uuid.New(), UserID: userID, Kind: models.ReportScheduled, Status: models.ReportPending,
			WindowStart: now.Add(-scheduledReportWindow), WindowEnd: now, OutputLang: p.OutputLang, CreatedAt: now,
		}
		if err := s.reports.Create(ctx, rep); err != nil {
			return err
		}
		_, err = s.enqueue(ctx, rep)
		return err
	})
}

// ReconcileOrphans fails the pending or running reports that no longer have an
// active job. A report and its job are created together, so a report without
// one (the job failed out of attempts, was recovered as stale, or was already
// purged by retention) will never finish, and would block the user's manual and
// scheduled reports forever. It runs on the worker's scheduler.
func (s *ReportService) ReconcileOrphans(ctx context.Context, now time.Time) error {
	if _, err := s.reports.FailOrphaned(ctx, ReportErrAIFailure, now); err != nil {
		return fmt.Errorf("reconcile orphaned reports: %w", err)
	}
	return nil
}

type reportJobPayload struct {
	ReportID uuid.UUID `json:"report_id"`
}

// Generate is the report.generate job handler.
func (s *ReportService) Generate(ctx context.Context, job models.Job) (any, error) {
	var payload reportJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, workers.Permanent(fmt.Errorf("decode payload: %w", err))
	}
	rep, err := s.reports.Get(ctx, payload.ReportID)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, workers.Permanent(err)
	}
	if err != nil {
		return nil, err
	}
	if rep.Status == models.ReportDone || rep.Status == models.ReportFailed {
		return map[string]string{"status": string(rep.Status)}, nil // already handled
	}
	if err := s.reports.MarkRunning(ctx, rep.ID); err != nil {
		return nil, err
	}

	code, err := s.generate(ctx, rep)
	if err == nil {
		return map[string]string{"status": "done"}, nil
	}
	if retryableReportError(code, err) && job.Attempts < job.MaxAttempts && ctx.Err() == nil {
		return nil, err // the queue retries with backoff; the report stays "running"
	}
	if code == "" {
		code = ReportErrAIFailure
	}
	if ferr := s.reports.Fail(context.WithoutCancel(ctx), rep.ID, code, s.now()); ferr != nil {
		return nil, ferr
	}
	return map[string]string{"status": "failed", "error": code, "detail": failureDetail(err)}, nil
}

// failureDetail is the error text kept in a failed job's result, so operators
// can see why a model's answer was rejected.
func failureDetail(err error) string {
	const limit = 1000
	msg := err.Error()
	if len(msg) > limit {
		return msg[:limit]
	}
	return msg
}

// retryableReportError tells infrastructure and transient provider failures
// (worth another attempt) from failures that would repeat (no model, no items,
// refusal, invalid output).
func retryableReportError(code string, err error) bool {
	switch code {
	case "":
		return true
	case ReportErrAIFailure:
		return ai.IsTemporary(err)
	default:
		return false
	}
}

// generate runs the pipeline. On failure it returns a report error code for
// failures that are the report's own (empty code = infrastructure error).
func (s *ReportService) generate(ctx context.Context, rep models.Report) (string, error) {
	reportModel, _, err := s.models.EffectiveModels(ctx, rep.UserID)
	if err != nil {
		return "", err
	}
	if reportModel == nil {
		return ReportErrNoModel, ErrModelUnavailable
	}

	topics, focus := []string{}, ""
	if p, err := s.prefs.Get(ctx, rep.UserID); err == nil {
		topics = p.Topics
	} else if !errors.Is(err, repositories.ErrNotFound) {
		return "", err
	}
	if rep.Focus != nil {
		focus = *rep.Focus
	}
	interest, err := s.interest(ctx, rep.UserID, focus)
	if err != nil {
		return "", err
	}

	candidates, total, err := s.candidates.Rank(ctx, repositories.CandidateQuery{
		UserID: rep.UserID, WindowStart: rep.WindowStart, WindowEnd: rep.WindowEnd,
		Interest: interest, Limit: reportCandidates, PerSource: reportPerSource, SkipCitedBefore: rep.CreatedAt,
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return ReportErrNoItems, errors.New("no new items in the report window")
	}
	extractArticles(ctx, s.extractor, s.candidates, candidates, reportExtractTop, s.now)

	_, prevSummary, err := s.reports.PreviousSummary(ctx, rep.UserID, rep.CreatedAt)
	if err != nil {
		return "", err
	}

	var out reportOutput
	userID := rep.UserID
	model, err := s.ai.GenerateJSON(ctx, *reportModel, models.PurposeReport, &userID, ai.ChatRequest{
		System:   reportSystemPrompt(rep.OutputLang),
		Messages: []ai.Message{{Role: ai.RoleUser, Content: buildReportPrompt(topics, focus, rep.WindowStart, rep.WindowEnd, prevSummary, candidates)}},
		Schema:   reportSchema,
	}, &out, func() error { return out.validate(len(candidates)) })
	if err != nil {
		return aiErrorCode(err), err
	}

	if prevSummary == "" {
		// Nothing to compare with: models sometimes fill what_changed anyway.
		out.WhatChanged = nil
	}
	markdown, refs := renderReport(out, rep.OutputLang, len(candidates))
	citations := make([]models.ReportCitation, len(refs))
	for i, ref := range refs {
		citations[i] = models.ReportCitation{ReportID: rep.ID, ItemID: candidates[ref-1].ItemID, Ref: ref}
	}
	now := s.now()
	title, summary, modelID, modelName := reportTitle(out), strings.TrimSpace(out.Summary), model.ID, model.Name
	rep.Title, rep.Summary, rep.ContentMD = &title, &summary, &markdown
	rep.ModelID, rep.ModelName, rep.ItemsConsidered, rep.CompletedAt = &modelID, &modelName, total, &now

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.reports.Complete(ctx, rep, citations); err != nil {
			return err
		}
		return s.followUpIdeas(ctx, rep)
	})
	return "", err
}

// interest picks the embedding that ranks the window: the focus of a manual
// report when given, otherwise the user's topics. Without an embedding model,
// ranking falls back to engagement and recency.
func (s *ReportService) interest(ctx context.Context, userID uuid.UUID, focus string) ([]float32, error) {
	if focus != "" {
		vectors, _, err := s.ai.Embed(ctx, models.PurposeEmbedding, []string{"Focus: " + focus})
		if errors.Is(err, ErrEmbeddingModelNotSet) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return vectors[0], nil
	}
	vector, _, err := s.topics.TopicsEmbedding(ctx, userID)
	return vector, err
}

// extractArticles fetches the article text of the first n candidates that have
// none yet, in parallel. Failures are cached so a paywalled page is not retried.
func extractArticles(ctx context.Context, extractor ArticleExtractor, store CandidateStore, candidates []repositories.Candidate, n int, now func() time.Time) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, extractConcurrency)
	for i := range candidates[:min(len(candidates), n)] {
		if candidates[i].ContentDone {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			c := &candidates[i]
			callCtx, cancel := context.WithTimeout(ctx, extractTimeout)
			text, err := extractor.Text(callCtx, c.URL)
			cancel()
			var textPtr, errPtr *string
			if err != nil {
				msg := clip(err.Error(), 300)
				errPtr = &msg
			} else {
				textPtr = &text
				c.ContentText = &text
			}
			_ = store.SetContent(context.WithoutCancel(ctx), c.ItemID, textPtr, errPtr, now())
		}()
	}
	wg.Wait()
}

// followUpIdeas queues the daily ideas after a scheduled report.
func (s *ReportService) followUpIdeas(ctx context.Context, rep models.Report) error {
	if rep.Kind != models.ReportScheduled {
		return nil
	}
	p, err := s.prefs.Get(ctx, rep.UserID)
	if err != nil || p.IdeasPerDay == 0 {
		return nil //nolint:nilerr // no preferences means no daily ideas
	}
	userID := rep.UserID
	_, err = s.queue.Enqueue(ctx, JobGenerateIdeas,
		map[string]any{"user_id": userID.String(), "count": p.IdeasPerDay, "report_id": rep.ID.String()},
		workers.EnqueueOptions{UserID: &userID, DedupeKey: "ideas:" + userID.String(), MaxAttempts: 3})
	return err
}

// aiErrorCode classifies a generation failure.
func aiErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrModelUnavailable):
		return ReportErrNoModel
	case errors.Is(err, ai.ErrRefused):
		return ReportErrRefused
	case strings.Contains(err.Error(), "did not match the schema"):
		return ReportErrInvalid
	default:
		return ReportErrAIFailure
	}
}
