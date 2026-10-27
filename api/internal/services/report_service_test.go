package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/pkg/ai"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

type reportDeps struct {
	reports    *mocks.MockReportStore
	candidates *mocks.MockCandidateStore
	prefs      *mocks.MockPreferenceStore
	topics     *mocks.MockTopicsReader
	models     *mocks.MockUserModels
	ai         *mocks.MockGenerationAI
	extractor  *mocks.MockArticleExtractor
	queue      *mocks.MockEnqueuer
}

func newReportService(t *testing.T) (*services.ReportService, reportDeps) {
	ctrl := gomock.NewController(t)
	d := reportDeps{
		reports: mocks.NewMockReportStore(ctrl), candidates: mocks.NewMockCandidateStore(ctrl), prefs: mocks.NewMockPreferenceStore(ctrl),
		topics: mocks.NewMockTopicsReader(ctrl), models: mocks.NewMockUserModels(ctrl), ai: mocks.NewMockGenerationAI(ctrl),
		extractor: mocks.NewMockArticleExtractor(ctrl), queue: mocks.NewMockEnqueuer(ctrl),
	}
	svc := services.NewReportService(inlineTx{}, d.reports, d.candidates, d.prefs, d.topics, d.models, d.ai, d.extractor, d.queue,
		func() time.Time { return fixedNow })
	return svc, d
}

func reportJob(id uuid.UUID, attempt int) models.Job {
	payload, _ := json.Marshal(map[string]string{"report_id": id.String()})
	return models.Job{Payload: payload, Attempts: attempt, MaxAttempts: 3}
}

func TestRequestReportOnePerUser(t *testing.T) {
	t.Parallel()
	svc, d := newReportService(t)
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}

	_, _, err := svc.Request(context.Background(), actor, services.ReportRequest{WindowHours: 0})
	requireCode(t, err, "VALIDATION_FAILED")

	d.prefs.EXPECT().EmailVerified(gomock.Any(), actor.UserID).Return(true, nil)
	d.prefs.EXPECT().Get(gomock.Any(), actor.UserID).Return(models.UserPreferences{OutputLang: "en"}, nil)
	d.reports.EXPECT().HasActive(gomock.Any(), actor.UserID).Return(true, nil)
	_, _, err = svc.Request(context.Background(), actor, services.ReportRequest{WindowHours: 24})
	requireCode(t, err, "REPORT_IN_PROGRESS")
}

func TestRequestReportNeedsVerifiedEmail(t *testing.T) {
	t.Parallel()
	svc, d := newReportService(t)
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}

	d.prefs.EXPECT().EmailVerified(gomock.Any(), actor.UserID).Return(false, nil)
	_, _, err := svc.Request(context.Background(), actor, services.ReportRequest{WindowHours: 24})
	requireCode(t, err, "EMAIL_NOT_VERIFIED")
}

func TestGenerateHappyPath(t *testing.T) {
	t.Parallel()
	svc, d := newReportService(t)
	userID, modelID, reportID := uuid.New(), uuid.New(), uuid.New()
	rep := models.Report{ID: reportID, UserID: userID, Kind: models.ReportScheduled, Status: models.ReportPending,
		WindowStart: fixedNow.Add(-24 * time.Hour), WindowEnd: fixedNow, OutputLang: "en", CreatedAt: fixedNow}
	items := []repositories.Candidate{
		{ItemID: uuid.New(), URL: "https://a.example/1", Title: "One", ContentDone: true},
		{ItemID: uuid.New(), URL: "https://a.example/2", Title: "Two"},
	}
	topicsVec := make([]float32, 768)

	d.reports.EXPECT().Get(gomock.Any(), reportID).Return(rep, nil)
	d.reports.EXPECT().MarkRunning(gomock.Any(), reportID).Return(nil)
	d.models.EXPECT().EffectiveModels(gomock.Any(), userID).Return(&modelID, nil, nil)
	d.prefs.EXPECT().Get(gomock.Any(), userID).Return(models.UserPreferences{Topics: []string{"AI"}, IdeasPerDay: 2}, nil).Times(2)
	d.topics.EXPECT().TopicsEmbedding(gomock.Any(), userID).Return(topicsVec, "nomic", nil)
	d.candidates.EXPECT().Rank(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, q repositories.CandidateQuery) ([]repositories.Candidate, int, error) {
		assert.Equal(t, topicsVec, q.Interest, "ranked by the user's topics")
		assert.Equal(t, fixedNow, q.SkipCitedBefore, "items cited by earlier reports are left out")
		return items, 57, nil
	})
	// Only the item without content is extracted.
	d.extractor.EXPECT().Text(gomock.Any(), "https://a.example/2").Return("", errors.New("paywall"))
	d.candidates.EXPECT().SetContent(gomock.Any(), items[1].ItemID, nil, gomock.Any(), fixedNow).Return(nil)
	d.reports.EXPECT().PreviousSummary(gomock.Any(), userID, fixedNow).Return("", "", nil)
	d.ai.EXPECT().GenerateJSON(gomock.Any(), modelID, models.PurposeReport, &userID, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ models.UsagePurpose, _ *uuid.UUID, req ai.ChatRequest, dst any, validate func() error) (services.ResolvedModel, error) {
			assert.Contains(t, req.System, "English")
			require.NoError(t, json.Unmarshal([]byte(`{"title":"T","summary":"S","what_changed":["invented change"],"sections":[{"heading":"A","body":"x [2]"},{"heading":"B","body":"y [1] [5]"}]}`), dst))
			require.NoError(t, validate())
			return services.ResolvedModel{ID: modelID, Name: "qwen3:14b"}, nil
		})
	d.reports.EXPECT().Complete(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r models.Report, cites []models.ReportCitation) error {
		assert.Equal(t, 57, r.ItemsConsidered)
		assert.NotContains(t, *r.ContentMD, "[5]", "invalid citations are dropped")
		assert.NotContains(t, *r.ContentMD, "invented change", "no previous report, so nothing changed")
		require.Len(t, cites, 2)
		assert.Equal(t, items[1].ItemID, cites[0].ItemID)
		assert.Equal(t, 2, cites[0].Ref)
		return nil
	})
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobGenerateIdeas, gomock.Any(), gomock.Any()).Return(uuid.New(), nil)

	res, err := svc.Generate(context.Background(), reportJob(reportID, 1))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"status": "done"}, res)
}

func TestGenerateFailures(t *testing.T) {
	t.Parallel()
	userID, modelID := uuid.New(), uuid.New()
	pending := func() models.Report {
		return models.Report{ID: uuid.New(), UserID: userID, Kind: models.ReportManual, Status: models.ReportPending, OutputLang: "pt-BR"}
	}

	t.Run("no model fails without retry", func(t *testing.T) {
		t.Parallel()
		svc, d := newReportService(t)
		rep := pending()
		d.reports.EXPECT().Get(gomock.Any(), rep.ID).Return(rep, nil)
		d.reports.EXPECT().MarkRunning(gomock.Any(), rep.ID).Return(nil)
		d.models.EXPECT().EffectiveModels(gomock.Any(), userID).Return(nil, nil, nil)
		d.reports.EXPECT().Fail(gomock.Any(), rep.ID, services.ReportErrNoModel, fixedNow).Return(nil)
		res, err := svc.Generate(context.Background(), reportJob(rep.ID, 1))
		require.NoError(t, err)
		assert.Equal(t, "failed", res.(map[string]string)["status"])
	})

	t.Run("no items", func(t *testing.T) {
		t.Parallel()
		svc, d := newReportService(t)
		rep := pending()
		d.reports.EXPECT().Get(gomock.Any(), rep.ID).Return(rep, nil)
		d.reports.EXPECT().MarkRunning(gomock.Any(), rep.ID).Return(nil)
		d.models.EXPECT().EffectiveModels(gomock.Any(), userID).Return(&modelID, nil, nil)
		d.prefs.EXPECT().Get(gomock.Any(), userID).Return(models.UserPreferences{}, repositories.ErrNotFound)
		d.topics.EXPECT().TopicsEmbedding(gomock.Any(), userID).Return(nil, "", nil)
		d.candidates.EXPECT().Rank(gomock.Any(), gomock.Any()).Return(nil, 0, nil)
		d.reports.EXPECT().Fail(gomock.Any(), rep.ID, services.ReportErrNoItems, fixedNow).Return(nil)
		_, err := svc.Generate(context.Background(), reportJob(rep.ID, 1))
		require.NoError(t, err)
	})

	t.Run("overloaded provider is retried, then fails on the last attempt", func(t *testing.T) {
		t.Parallel()
		for attempt, wantRetry := range map[int]bool{1: true, 3: false} {
			svc, d := newReportService(t)
			rep := pending()
			d.reports.EXPECT().Get(gomock.Any(), rep.ID).Return(rep, nil)
			d.reports.EXPECT().MarkRunning(gomock.Any(), rep.ID).Return(nil)
			d.models.EXPECT().EffectiveModels(gomock.Any(), userID).Return(&modelID, nil, nil)
			d.prefs.EXPECT().Get(gomock.Any(), userID).Return(models.UserPreferences{}, repositories.ErrNotFound)
			d.topics.EXPECT().TopicsEmbedding(gomock.Any(), userID).Return(nil, "", nil)
			d.candidates.EXPECT().Rank(gomock.Any(), gomock.Any()).Return([]repositories.Candidate{{ItemID: uuid.New(), ContentDone: true}}, 1, nil)
			d.reports.EXPECT().PreviousSummary(gomock.Any(), userID, gomock.Any()).Return("", "", nil)
			d.ai.EXPECT().GenerateJSON(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(services.ResolvedModel{}, &ai.APIError{Provider: ai.KindAnthropic, Status: 529, Message: "Overloaded"})
			if !wantRetry {
				d.reports.EXPECT().Fail(gomock.Any(), rep.ID, services.ReportErrAIFailure, fixedNow).Return(nil)
			}
			_, err := svc.Generate(context.Background(), reportJob(rep.ID, attempt))
			assert.Equal(t, wantRetry, err != nil, "attempt %d", attempt)
		}
	})

	t.Run("finished reports are left alone", func(t *testing.T) {
		t.Parallel()
		svc, d := newReportService(t)
		rep := pending()
		rep.Status = models.ReportDone
		d.reports.EXPECT().Get(gomock.Any(), rep.ID).Return(rep, nil)
		res, err := svc.Generate(context.Background(), reportJob(rep.ID, 2))
		require.NoError(t, err)
		assert.Equal(t, "done", res.(map[string]string)["status"])
	})
}
