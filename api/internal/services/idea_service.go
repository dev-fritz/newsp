package services

//go:generate go tool mockgen -source=idea_service.go -destination=../../tests/mocks/idea_mocks.go -package=mocks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/ai"
)

// JobDeepenIdea writes a deeper version of one idea.
const JobDeepenIdea = "idea.deepen"

// Idea error codes returned in job results; the web app translates them.
const (
	IdeaErrNoModel = "IDEA_MODEL_NOT_SET"
	IdeaErrNoItems = "NO_ITEMS"
	// IdeaErrNoChanges: a deepen returned the idea as it was, even after a retry.
	IdeaErrNoChanges = "NO_CHANGES"
)

// Idea pipeline tuning.
const (
	ideaCandidates     = 30
	ideaPerSource      = 8
	ideaExtractTop     = 10
	ideaExistingTitles = 50
	ideaSimilarLimit   = 3
	maxIdeasPerRequest = 5
	maxIdeaItems       = 5 // items a caller can pick to seed a generation
	deepenAttempts     = 2 // a deepen that changes nothing is asked once more
)

var errIdeaNotFound = apperrors.NotFound("IDEA_NOT_FOUND", "idea not found")

// IdeaStore persists ideas.
type IdeaStore interface {
	LockUser(ctx context.Context, userID uuid.UUID) error
	Create(ctx context.Context, idea models.Idea, v models.IdeaVersion, embedding []float32, evidence []models.IdeaEvidence) error
	AddVersion(ctx context.Context, idea models.Idea, v models.IdeaVersion, embedding []float32, evidence []models.IdeaEvidence) error
	GetForUser(ctx context.Context, id, userID uuid.UUID) (models.Idea, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (models.Idea, error)
	SetStatus(ctx context.Context, id uuid.UUID, status models.IdeaStatus, now time.Time) error
	List(ctx context.Context, f repositories.IdeaFilter) ([]repositories.IdeaListItem, int, error)
	Version(ctx context.Context, ideaID uuid.UUID, version int) (models.IdeaVersion, error)
	Versions(ctx context.Context, ideaID uuid.UUID) ([]models.IdeaVersion, error)
	Evidence(ctx context.Context, versionID uuid.UUID) ([]repositories.EvidenceItem, error)
	Similar(ctx context.Context, userID uuid.UUID, vector []float32, minSimilarity float32, limit int) ([]repositories.SimilarIdea, error)
	RecentTitles(ctx context.Context, userID uuid.UUID, limit int) ([]models.Idea, error)
	FindByTitle(ctx context.Context, userID uuid.UUID, title string) (models.Idea, error)
}

// IdeaReports reads the report an idea generation starts from.
type IdeaReports interface {
	GetForUser(ctx context.Context, id, userID uuid.UUID) (models.Report, error)
}

// IdeaGenerationRequest asks for new ideas.
type IdeaGenerationRequest struct {
	Count      int
	Focus      string
	WindowDays int
	// ReportID starts from a finished report: its cited items first, then its
	// window and focus. Focus and WindowDays are ignored.
	ReportID *uuid.UUID
	// ItemIDs starts from news items the caller picked: they are cited first, then
	// the window and focus fill the rest. Not allowed with ReportID.
	ItemIDs []uuid.UUID
}

// DeepenRequest asks for a deeper version of an idea.
type DeepenRequest struct {
	Aspects     []string
	Instruction string
}

// IdeaEdit replaces the content of an idea by hand.
type IdeaEdit struct {
	Title       string
	Summary     string
	Difficulty  models.Difficulty
	EffortWeeks int
	Content     models.IdeaContent
}

// IdeaVersionDetail is a version with its evidence.
type IdeaVersionDetail struct {
	Version  models.IdeaVersion
	Content  models.IdeaContent
	Evidence []repositories.EvidenceItem
}

// IdeaDetail is an idea with its current version and history.
type IdeaDetail struct {
	Idea     models.Idea
	Current  IdeaVersionDetail
	Versions []models.IdeaVersion
}

// GenerationResult summarizes a generation run.
type GenerationResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	// Detail is the underlying error, for operators diagnosing a model.
	Detail  string      `json:"detail,omitempty"`
	Created []uuid.UUID `json:"created"`
	Merged  []uuid.UUID `json:"merged"`
}

// IdeaService generates, deduplicates, versions and serves project ideas.
type IdeaService struct {
	tx         TxRunner
	ideas      IdeaStore
	candidates CandidateStore
	reports    IdeaReports
	prefs      PreferenceStore
	topics     TopicsReader
	models     UserModels
	ai         GenerationAI
	extractor  ArticleExtractor
	settings   SettingsStore
	queue      Enqueuer
	now        func() time.Time
}

// NewIdeaService builds an IdeaService.
func NewIdeaService(tx TxRunner, ideas IdeaStore, candidates CandidateStore, reports IdeaReports, prefs PreferenceStore, topics TopicsReader,
	userModels UserModels, gen GenerationAI, extractor ArticleExtractor, settings SettingsStore, queue Enqueuer, now func() time.Time,
) *IdeaService {
	return &IdeaService{
		tx: tx, ideas: ideas, candidates: candidates, reports: reports, prefs: prefs, topics: topics, models: userModels, ai: gen,
		extractor: extractor, settings: settings, queue: queue, now: now,
	}
}

// RequestGeneration queues an idea generation for the caller. Requests while
// one is running return the running job.
func (s *IdeaService) RequestGeneration(ctx context.Context, actor auth.Identity, in IdeaGenerationRequest) (uuid.UUID, error) {
	fields := map[string]string{}
	if in.Count < 1 || in.Count > maxIdeasPerRequest {
		fields["count"] = "max"
	}
	if in.ReportID == nil && (in.WindowDays < 1 || in.WindowDays > 30) {
		fields["window_days"] = "max"
	}
	switch {
	case len(in.ItemIDs) > maxIdeaItems:
		fields["item_ids"] = "max"
	case in.ReportID != nil && len(in.ItemIDs) > 0:
		fields["item_ids"] = "conflict"
	}
	if len(fields) > 0 {
		return uuid.Nil, apperrors.Validation("invalid request", fields)
	}
	userID := actor.UserID
	if err := requireVerifiedEmail(ctx, s.prefs, userID); err != nil {
		return uuid.Nil, err
	}
	if in.ReportID != nil {
		rep, err := s.reports.GetForUser(ctx, *in.ReportID, userID)
		if errors.Is(err, repositories.ErrNotFound) {
			return uuid.Nil, apperrors.NotFound("REPORT_NOT_FOUND", "report not found")
		}
		if err != nil {
			return uuid.Nil, fmt.Errorf("request ideas: %w", err)
		}
		if rep.Status != models.ReportDone {
			return uuid.Nil, apperrors.Validation("the report is not finished", map[string]string{"report_id": "not_ready"})
		}
	}
	if len(in.ItemIDs) > 0 {
		found, err := s.candidates.ByIDs(ctx, in.ItemIDs)
		if err != nil {
			return uuid.Nil, fmt.Errorf("request ideas: %w", err)
		}
		if len(found) != len(in.ItemIDs) {
			return uuid.Nil, apperrors.Validation("unknown items", map[string]string{"item_ids": "not_found"})
		}
	}
	id, err := s.queue.Enqueue(ctx, JobGenerateIdeas, generationPayload{
		UserID: userID, Count: in.Count, Focus: strings.TrimSpace(in.Focus), WindowDays: in.WindowDays, ReportID: in.ReportID,
		ItemIDs: in.ItemIDs,
	}, workers.EnqueueOptions{UserID: &userID, DedupeKey: "ideas:" + userID.String(), MaxAttempts: 3})
	if err != nil {
		return uuid.Nil, fmt.Errorf("request ideas: %w", err)
	}
	return id, nil
}

type generationPayload struct {
	UserID     uuid.UUID   `json:"user_id"`
	Count      int         `json:"count"`
	Focus      string      `json:"focus,omitempty"`
	WindowDays int         `json:"window_days,omitempty"`
	ReportID   *uuid.UUID  `json:"report_id,omitempty"`
	ItemIDs    []uuid.UUID `json:"item_ids,omitempty"`
}

// Generate is the ideas.generate job handler. Each candidate is deduplicated
// against the user's ideas (discarded ones included) before it is stored:
// a match becomes a new version of the existing idea instead of a duplicate.
func (s *IdeaService) Generate(ctx context.Context, job models.Job) (any, error) {
	var p generationPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil || p.UserID == uuid.Nil {
		return nil, workers.Permanent(fmt.Errorf("decode payload: %w", err))
	}
	p.Count = min(max(p.Count, 1), maxIdeasPerRequest)
	if p.WindowDays <= 0 {
		p.WindowDays = 7
	}

	_, ideaModel, err := s.models.EffectiveModels(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if ideaModel == nil {
		return GenerationResult{Status: "failed", Error: IdeaErrNoModel}, nil
	}
	prefs := DefaultPreferences(p.UserID, "pt-BR")
	if stored, err := s.prefs.Get(ctx, p.UserID); err == nil {
		prefs = stored
	} else if !errors.Is(err, repositories.ErrNotFound) {
		return nil, err
	}

	now := s.now()
	focus, from, to := p.Focus, now.Add(-time.Duration(p.WindowDays)*24*time.Hour), now
	var cited []repositories.Candidate
	if p.ReportID != nil {
		rep, err := s.reports.GetForUser(ctx, *p.ReportID, p.UserID)
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, workers.Permanent(err)
		}
		if err != nil {
			return nil, err
		}
		focus, from, to = "", rep.WindowStart, rep.WindowEnd
		if rep.Focus != nil {
			focus = *rep.Focus
		}
		if cited, err = s.candidates.Cited(ctx, rep.ID); err != nil {
			return nil, err
		}
	} else if len(p.ItemIDs) > 0 {
		// Items deleted since the request are simply absent.
		if cited, err = s.candidates.ByIDs(ctx, p.ItemIDs); err != nil {
			return nil, err
		}
	}
	candidates, err := s.sources(ctx, p.UserID, focus, nil, from, to, cited)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return GenerationResult{Status: "failed", Error: IdeaErrNoItems}, nil
	}
	existing, err := s.ideas.RecentTitles(ctx, p.UserID, ideaExistingTitles)
	if err != nil {
		return nil, err
	}

	start := startingSources{n: leadingCited(cited, candidates), picked: p.ReportID == nil}

	var out generationOutput
	userID := p.UserID
	model, err := s.ai.GenerateJSON(ctx, *ideaModel, models.PurposeIdea, &userID, ai.ChatRequest{
		System:   ideaSystemPrompt(prefs.OutputLang),
		Messages: []ai.Message{{Role: ai.RoleUser, Content: buildGenerationPrompt(p.Count, prefs.OutputLang, prefs.Topics, focus, existing, candidates, start)}},
		Schema:   generationSchema,
	}, &out, func() error {
		if len(out.Ideas) == 0 {
			return errors.New("ideas must not be empty")
		}
		for i := range out.Ideas {
			if err := out.Ideas[i].validate(len(candidates), true); err != nil {
				return fmt.Errorf("idea %d: %w", i+1, err)
			}
			if start.n > 0 && !citesStarting(out.Ideas[i].Evidence, start.n) {
				return fmt.Errorf("idea %d: evidence must cite at least one of the starting sources %s", i+1, sourceRefs(start.n))
			}
		}
		return nil
	})
	if err != nil {
		return s.generationFailure(job, err)
	}

	threshold, err := s.threshold(ctx)
	if err != nil {
		return nil, err
	}
	result := GenerationResult{Status: "done", Created: []uuid.UUID{}, Merged: []uuid.UUID{}}
	for _, draft := range out.Ideas[:min(len(out.Ideas), p.Count)] {
		ideaID, merged, err := s.store(ctx, p.UserID, prefs.OutputLang, *ideaModel, model, threshold, draft, candidates)
		if err != nil {
			return nil, err
		}
		if merged {
			result.Merged = append(result.Merged, ideaID)
		} else {
			result.Created = append(result.Created, ideaID)
		}
	}
	return result, nil
}

func (s *IdeaService) generationFailure(job models.Job, err error) (any, error) {
	code := aiErrorCode(err)
	if retryableReportError(code, err) && job.Attempts < job.MaxAttempts {
		return nil, err
	}
	return GenerationResult{Status: "failed", Error: code, Detail: failureDetail(err)}, nil
}

func (s *IdeaService) threshold(ctx context.Context) (float32, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return 0, err
	}
	return settings.IdeaSimilarityThreshold, nil
}

// sources ranks recent items for an idea prompt and extracts the top ones.
// interest overrides the focus/topics ranking (used when deepening an idea).
// sources picks the items an idea prompt cites: the given cited ones first (a
// report's), then the best ranked of the window up to the usual limit.
func (s *IdeaService) sources(ctx context.Context, userID uuid.UUID, focus string, interest []float32, from, to time.Time,
	cited []repositories.Candidate,
) ([]repositories.Candidate, error) {
	if interest == nil {
		var err error
		if interest, err = s.interestFor(ctx, userID, focus); err != nil {
			return nil, err
		}
	}
	ranked, _, err := s.candidates.Rank(ctx, repositories.CandidateQuery{
		UserID: userID, WindowStart: from, WindowEnd: to, Interest: interest, Limit: ideaCandidates, PerSource: ideaPerSource,
	})
	if err != nil {
		return nil, err
	}
	candidates := withCited(cited, ranked, ideaCandidates)
	extractArticles(ctx, s.extractor, s.candidates, candidates, ideaExtractTop, s.now)
	return candidates, nil
}

// leadingCited counts the cited items at the head of candidates (withCited puts
// them there; one the ranking dropped is simply not counted).
func leadingCited(cited, candidates []repositories.Candidate) int {
	ids := make(map[uuid.UUID]bool, len(cited))
	for _, c := range cited {
		ids[c.ItemID] = true
	}
	n := 0
	for n < len(candidates) && ids[candidates[n].ItemID] {
		n++
	}
	return n
}

// withCited puts the cited items first and fills up to limit with the ranked
// ones not already there.
func withCited(cited, ranked []repositories.Candidate, limit int) []repositories.Candidate {
	if len(cited) == 0 {
		return ranked
	}
	out := make([]repositories.Candidate, 0, limit)
	seen := make(map[uuid.UUID]bool, limit)
	for _, list := range [][]repositories.Candidate{cited, ranked} {
		for _, c := range list {
			if len(out) == limit {
				return out
			}
			if !seen[c.ItemID] {
				seen[c.ItemID] = true
				out = append(out, c)
			}
		}
	}
	return out
}

func (s *IdeaService) interestFor(ctx context.Context, userID uuid.UUID, focus string) ([]float32, error) {
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

// store deduplicates one candidate and saves it as a new idea or as a new
// version of the idea it duplicates. It reports whether it merged.
func (s *IdeaService) store(ctx context.Context, userID uuid.UUID, lang string, modelID uuid.UUID, model ResolvedModel,
	threshold float32, draft ideaDraft, candidates []repositories.Candidate,
) (uuid.UUID, bool, error) {
	mapper := newEvidenceMapper(nil, candidates)
	title, summary, difficulty, effort, content := mapper.toVersion(draft)
	vector, embeddingModel, err := s.embedIdea(ctx, title, summary, content)
	if err != nil {
		return uuid.Nil, false, err
	}

	for attempt := 0; attempt < 2; attempt++ {
		match, similarity, judged, err := s.findDuplicate(ctx, userID, modelID, vector, threshold, title, draft)
		if err != nil {
			return uuid.Nil, false, err
		}
		if match != nil {
			if err := s.merge(ctx, userID, lang, modelID, *match, similarity, draft, candidates); err != nil {
				return uuid.Nil, false, err
			}
			return match.ID, true, nil
		}

		now := s.now()
		idea := models.Idea{ID: uuid.New(), UserID: userID, Status: models.IdeaNew, Title: title, Summary: summary, CreatedAt: now}
		if embeddingModel != "" {
			idea.EmbeddingModel = &embeddingModel
		}
		version, err := s.newVersion(idea.ID, 1, models.OriginGenerated, title, summary, difficulty, effort, content, &model, nil, nil, now)
		if err != nil {
			return uuid.Nil, false, err
		}
		created := false
		err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := s.ideas.LockUser(ctx, userID); err != nil {
				return err
			}
			// Another run may have stored a twin since the check above. Ideas
			// the judge already called distinct do not count, or a candidate
			// close to one of them could never be stored.
			if vector != nil {
				twins, err := s.ideas.Similar(ctx, userID, vector, threshold, len(judged)+1)
				if err != nil || hasUnjudged(twins, judged) {
					return err
				}
			}
			created = true
			return s.ideas.Create(ctx, idea, version, vector, mapper.evidence)
		})
		if err != nil {
			return uuid.Nil, false, err
		}
		if created {
			return idea.ID, false, nil
		}
	}
	return uuid.Nil, false, errors.New("idea kept colliding with concurrent writes")
}

// hasUnjudged reports whether twins holds an idea outside judged.
func hasUnjudged(twins []repositories.SimilarIdea, judged []uuid.UUID) bool {
	for _, twin := range twins {
		if !slices.Contains(judged, twin.Idea.ID) {
			return true
		}
	}
	return false
}

// findDuplicate returns the existing idea that the candidate duplicates, if
// any. When there is none, judged lists the ideas the judge called distinct.
func (s *IdeaService) findDuplicate(ctx context.Context, userID, modelID uuid.UUID, vector []float32, threshold float32,
	title string, draft ideaDraft,
) (match *models.Idea, similarity *float32, judged []uuid.UUID, err error) {
	if vector == nil { // no embedding model: exact title is the only signal we have
		idea, err := s.ideas.FindByTitle(ctx, userID, title)
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, nil, nil, nil
		}
		if err != nil {
			return nil, nil, nil, err
		}
		return &idea, nil, nil, nil
	}
	similar, err := s.ideas.Similar(ctx, userID, vector, threshold, ideaSimilarLimit)
	if err != nil || len(similar) == 0 {
		return nil, nil, nil, err
	}

	var verdict judgeOutput
	uid := userID
	_, err = s.ai.GenerateJSON(ctx, modelID, models.PurposeIdeaJudge, &uid, ai.ChatRequest{
		System:    judgeSystemPrompt(),
		Messages:  []ai.Message{{Role: ai.RoleUser, Content: buildJudgePrompt(draft, similar)}},
		Schema:    judgeSchema,
		MaxTokens: 1000,
	}, &verdict, func() error {
		if verdict.Match < 0 || verdict.Match > len(similar) {
			return fmt.Errorf("match must be between 0 and %d", len(similar))
		}
		return nil
	})
	if err != nil {
		// Unsure means "probably the same": a near-duplicate merged is better
		// than a duplicate created.
		best := similar[0]
		return &best.Idea, &best.Similarity, nil, nil //nolint:nilerr // a failed judge falls back to merging on purpose
	}
	if verdict.Match == 0 {
		for _, sim := range similar {
			judged = append(judged, sim.Idea.ID)
		}
		return nil, nil, judged, nil
	}
	m := similar[verdict.Match-1]
	return &m.Idea, &m.Similarity, nil, nil
}

// merge writes the next version of an existing idea with what the candidate adds.
func (s *IdeaService) merge(ctx context.Context, userID uuid.UUID, lang string, modelID uuid.UUID, idea models.Idea,
	similarity *float32, draft ideaDraft, candidates []repositories.Candidate,
) error {
	current, err := s.versionDetail(ctx, idea.ID, idea.CurrentVersion)
	if err != nil {
		return err
	}
	var out revisionOutput
	uid := userID
	model, err := s.ai.GenerateJSON(ctx, modelID, models.PurposeIdeaMerge, &uid, ai.ChatRequest{
		System:   revisionSystemPrompt(lang),
		Messages: []ai.Message{{Role: ai.RoleUser, Content: buildMergePrompt(current.Version, current.Content, draft, candidates)}},
		Schema:   revisionSchema,
	}, &out, func() error { return out.Idea.validate(len(candidates), false) })
	if err != nil {
		return err
	}
	return s.addVersion(ctx, idea.ID, models.OriginMerged, current, out, candidates, &model, similarity, nil, true)
}

// addVersion stores a revision as the next version of an idea.
func (s *IdeaService) addVersion(ctx context.Context, ideaID uuid.UUID, origin models.VersionOrigin, current IdeaVersionDetail,
	out revisionOutput, candidates []repositories.Candidate, model *ResolvedModel, similarity *float32, createdBy *uuid.UUID, merged bool,
) error {
	mapper := newEvidenceMapper(current.Evidence, candidates)
	title, summary, difficulty, effort, content := mapper.toVersion(out.Idea)
	vector, embeddingModel, err := s.embedIdea(ctx, title, summary, content)
	if err != nil {
		return err
	}
	note := strings.TrimSpace(out.ChangeNote)
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		idea, err := s.ideas.GetForUpdate(ctx, ideaID)
		if err != nil {
			return err
		}
		version, err := s.newVersion(ideaID, idea.CurrentVersion+1, origin, title, summary, difficulty, effort, content, model, similarity, createdBy, s.now())
		if err != nil {
			return err
		}
		if note != "" {
			version.ChangeNote = &note
		}
		if merged {
			idea.MergedCandidates++
		}
		if embeddingModel != "" {
			idea.EmbeddingModel = &embeddingModel
		}
		return s.ideas.AddVersion(ctx, idea, version, vector, mapper.evidence)
	})
}

func (s *IdeaService) newVersion(ideaID uuid.UUID, number int, origin models.VersionOrigin, title, summary string, difficulty models.Difficulty,
	effort int, content models.IdeaContent, model *ResolvedModel, similarity *float32, createdBy *uuid.UUID, now time.Time,
) (models.IdeaVersion, error) {
	raw, err := json.Marshal(content)
	if err != nil {
		return models.IdeaVersion{}, fmt.Errorf("encode idea content: %w", err)
	}
	v := models.IdeaVersion{
		ID: uuid.New(), IdeaID: ideaID, Version: number, Origin: origin, Title: title, Summary: summary,
		Difficulty: difficulty, Content: raw, Similarity: similarity, CreatedBy: createdBy, CreatedAt: now,
	}
	if effort > 0 {
		v.EffortWeeks = &effort
	}
	if model != nil && model.ID != uuid.Nil {
		v.ModelID, v.ModelName = &model.ID, &model.Name
	}
	return v, nil
}

// embedIdea embeds an idea; without an embedding model it returns nil.
func (s *IdeaService) embedIdea(ctx context.Context, title, summary string, content models.IdeaContent) ([]float32, string, error) {
	vectors, model, err := s.ai.Embed(ctx, models.PurposeEmbedding, []string{embeddingText(title, summary, content)})
	if errors.Is(err, ErrEmbeddingModelNotSet) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	return vectors[0], model, nil
}

func (s *IdeaService) versionDetail(ctx context.Context, ideaID uuid.UUID, number int) (IdeaVersionDetail, error) {
	v, err := s.ideas.Version(ctx, ideaID, number)
	if err != nil {
		return IdeaVersionDetail{}, err
	}
	var content models.IdeaContent
	if err := json.Unmarshal(v.Content, &content); err != nil {
		return IdeaVersionDetail{}, fmt.Errorf("decode idea content: %w", err)
	}
	evidence, err := s.ideas.Evidence(ctx, v.ID)
	if err != nil {
		return IdeaVersionDetail{}, err
	}
	return IdeaVersionDetail{Version: v, Content: content, Evidence: evidence}, nil
}

// RequestDeepen queues a deeper version of one of the caller's ideas.
func (s *IdeaService) RequestDeepen(ctx context.Context, actor auth.Identity, ideaID uuid.UUID, in DeepenRequest) (uuid.UUID, error) {
	userID := actor.UserID
	if err := requireVerifiedEmail(ctx, s.prefs, userID); err != nil {
		return uuid.Nil, err
	}
	if _, err := s.owned(ctx, actor, ideaID); err != nil {
		return uuid.Nil, err
	}
	id, err := s.queue.Enqueue(ctx, JobDeepenIdea, deepenPayload{
		IdeaID: ideaID, UserID: userID, Aspects: in.Aspects, Instruction: strings.TrimSpace(in.Instruction),
	}, workers.EnqueueOptions{UserID: &userID, DedupeKey: "idea.deepen:" + ideaID.String(), MaxAttempts: 3})
	if err != nil {
		return uuid.Nil, fmt.Errorf("request deepen: %w", err)
	}
	return id, nil
}

type deepenPayload struct {
	IdeaID      uuid.UUID `json:"idea_id"`
	UserID      uuid.UUID `json:"user_id"`
	Aspects     []string  `json:"aspects"`
	Instruction string    `json:"instruction,omitempty"`
}

// Deepen is the idea.deepen job handler.
func (s *IdeaService) Deepen(ctx context.Context, job models.Job) (any, error) {
	var p deepenPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, workers.Permanent(fmt.Errorf("decode payload: %w", err))
	}
	idea, err := s.ideas.GetForUser(ctx, p.IdeaID, p.UserID)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, workers.Permanent(err)
	}
	if err != nil {
		return nil, err
	}
	_, ideaModel, err := s.models.EffectiveModels(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if ideaModel == nil {
		return GenerationResult{Status: "failed", Error: IdeaErrNoModel}, nil
	}
	lang := "pt-BR"
	if prefs, err := s.prefs.Get(ctx, p.UserID); err == nil {
		lang = prefs.OutputLang
	}
	current, err := s.versionDetail(ctx, idea.ID, idea.CurrentVersion)
	if err != nil {
		return nil, err
	}
	now := s.now()
	interest, _, err := s.embedIdea(ctx, current.Version.Title, current.Version.Summary, current.Content)
	if err != nil {
		return nil, err
	}
	candidates, err := s.sources(ctx, p.UserID, "", interest, now.Add(-7*24*time.Hour), now, nil)
	if err != nil {
		return nil, err
	}

	userID := p.UserID
	prompt := buildDeepenPrompt(current.Version, current.Content, p.Aspects, p.Instruction, candidates)
	var (
		out   revisionOutput
		model ResolvedModel
	)
	// Small models often hand the idea back as it was; ask once more, firmly,
	// then report it instead of storing a version identical to the current one.
	for attempt := 1; ; attempt++ {
		out = revisionOutput{}
		model, err = s.ai.GenerateJSON(ctx, *ideaModel, models.PurposeIdeaDeepen, &userID, ai.ChatRequest{
			System:   revisionSystemPrompt(lang),
			Messages: []ai.Message{{Role: ai.RoleUser, Content: prompt}},
			Schema:   revisionSchema,
		}, &out, func() error { return out.Idea.validate(len(candidates), false) })
		if err != nil {
			return s.generationFailure(job, err)
		}
		title, summary, difficulty, effort, content := newEvidenceMapper(current.Evidence, candidates).toVersion(out.Idea)
		if !unchanged(current, ideaText{Title: title, Summary: summary, Difficulty: difficulty, EffortWeeks: effort, Content: content}) {
			break
		}
		if attempt == deepenAttempts {
			return GenerationResult{Status: "failed", Error: IdeaErrNoChanges, Created: []uuid.UUID{}, Merged: []uuid.UUID{}}, nil
		}
		prompt += deepenRetryNote
	}
	if err := s.addVersion(ctx, idea.ID, models.OriginDeepened, current, out, candidates, &model, nil, &userID, false); err != nil {
		return nil, err
	}
	return GenerationResult{Status: "done", Created: []uuid.UUID{}, Merged: []uuid.UUID{idea.ID}}, nil
}

func (s *IdeaService) owned(ctx context.Context, actor auth.Identity, id uuid.UUID) (models.Idea, error) {
	idea, err := s.ideas.GetForUser(ctx, id, actor.UserID)
	if errors.Is(err, repositories.ErrNotFound) {
		return models.Idea{}, errIdeaNotFound
	}
	if err != nil {
		return models.Idea{}, fmt.Errorf("get idea: %w", err)
	}
	return idea, nil
}

// List returns a page of the caller's ideas.
func (s *IdeaService) List(ctx context.Context, actor auth.Identity, status, query string, limit, offset int) ([]repositories.IdeaListItem, int, error) {
	items, total, err := s.ideas.List(ctx, repositories.IdeaFilter{
		UserID: actor.UserID, Status: status, Query: strings.TrimSpace(query), Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list ideas: %w", err)
	}
	return items, total, nil
}

// Get returns one of the caller's ideas with its history.
func (s *IdeaService) Get(ctx context.Context, actor auth.Identity, id uuid.UUID) (IdeaDetail, error) {
	idea, err := s.owned(ctx, actor, id)
	if err != nil {
		return IdeaDetail{}, err
	}
	current, err := s.versionDetail(ctx, idea.ID, idea.CurrentVersion)
	if err != nil {
		return IdeaDetail{}, fmt.Errorf("get idea: %w", err)
	}
	versions, err := s.ideas.Versions(ctx, idea.ID)
	if err != nil {
		return IdeaDetail{}, fmt.Errorf("get idea: %w", err)
	}
	return IdeaDetail{Idea: idea, Current: current, Versions: versions}, nil
}

// GetVersion returns one version of one of the caller's ideas, for diffs.
func (s *IdeaService) GetVersion(ctx context.Context, actor auth.Identity, id uuid.UUID, number int) (IdeaVersionDetail, error) {
	if _, err := s.owned(ctx, actor, id); err != nil {
		return IdeaVersionDetail{}, err
	}
	detail, err := s.versionDetail(ctx, id, number)
	if errors.Is(err, repositories.ErrNotFound) {
		return IdeaVersionDetail{}, apperrors.NotFound("IDEA_VERSION_NOT_FOUND", "version not found")
	}
	if err != nil {
		return IdeaVersionDetail{}, fmt.Errorf("get idea version: %w", err)
	}
	return detail, nil
}

// SetStatus triages an idea. Discarded ideas keep blocking duplicates.
func (s *IdeaService) SetStatus(ctx context.Context, actor auth.Identity, id uuid.UUID, status models.IdeaStatus) (models.Idea, error) {
	idea, err := s.owned(ctx, actor, id)
	if err != nil {
		return models.Idea{}, err
	}
	now := s.now()
	if err := s.ideas.SetStatus(ctx, id, status, now); err != nil {
		return models.Idea{}, fmt.Errorf("set idea status: %w", err)
	}
	idea.Status, idea.UpdatedAt = status, now
	return idea, nil
}

// Edit stores a hand-written version of an idea. Evidence carries over.
func (s *IdeaService) Edit(ctx context.Context, actor auth.Identity, id uuid.UUID, in IdeaEdit) (IdeaDetail, error) {
	idea, err := s.owned(ctx, actor, id)
	if err != nil {
		return IdeaDetail{}, err
	}
	current, err := s.versionDetail(ctx, idea.ID, idea.CurrentVersion)
	if err != nil {
		return IdeaDetail{}, fmt.Errorf("edit idea: %w", err)
	}
	draft := ideaDraft{
		Title: in.Title, Summary: in.Summary, WhyNow: in.Content.WhyNow, Audience: in.Content.Audience, How: in.Content.How,
		Differentiator: in.Content.Differentiator, Stack: in.Content.Stack, ArchitectureSteps: in.Content.ArchitectureSteps,
		MVPPlan: in.Content.MVPPlan, Monetization: in.Content.Monetization, Competitors: in.Content.Competitors,
		Difficulty: in.Difficulty, EffortWeeks: in.EffortWeeks,
	}
	if err := draft.validate(0, false); err != nil {
		return IdeaDetail{}, apperrors.Validation(err.Error(), nil)
	}
	userID := actor.UserID
	if err := s.addVersion(ctx, idea.ID, models.OriginManual, current, revisionOutput{Idea: draft}, nil, nil, nil, &userID, false); err != nil {
		return IdeaDetail{}, fmt.Errorf("edit idea: %w", err)
	}
	return s.Get(ctx, actor, id)
}
