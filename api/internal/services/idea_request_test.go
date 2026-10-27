package services_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

type ideaRequestDeps struct {
	reports    *mocks.MockIdeaReports
	prefs      *mocks.MockPreferenceStore
	candidates *mocks.MockCandidateStore
	queue      *mocks.MockEnqueuer
}

func newIdeaRequestService(t *testing.T) (*services.IdeaService, ideaRequestDeps) {
	ctrl := gomock.NewController(t)
	d := ideaRequestDeps{
		reports: mocks.NewMockIdeaReports(ctrl), prefs: mocks.NewMockPreferenceStore(ctrl), candidates: mocks.NewMockCandidateStore(ctrl), queue: mocks.NewMockEnqueuer(ctrl),
	}
	svc := services.NewIdeaService(inlineTx{}, nil, d.candidates, d.reports, d.prefs, nil, nil, nil, nil, nil, d.queue,
		func() time.Time { return fixedNow })
	return svc, d
}

func requireFieldError(t *testing.T, err error, field, code string) {
	t.Helper()
	requireCode(t, err, "VALIDATION_FAILED")
	appErr, _ := apperrors.As(err)
	assert.Equal(t, code, appErr.Fields[field])
}

func TestRequestGenerationItemIDsValidation(t *testing.T) {
	t.Parallel()
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}
	ids := func(n int) []uuid.UUID {
		out := make([]uuid.UUID, n)
		for i := range out {
			out[i] = uuid.New()
		}
		return out
	}

	t.Run("more than five items", func(t *testing.T) {
		t.Parallel()
		svc, _ := newIdeaRequestService(t)
		_, err := svc.RequestGeneration(context.Background(), actor, services.IdeaGenerationRequest{Count: 1, WindowDays: 7, ItemIDs: ids(6)})
		requireFieldError(t, err, "item_ids", "max")
	})

	t.Run("items conflict with a report", func(t *testing.T) {
		t.Parallel()
		svc, _ := newIdeaRequestService(t)
		reportID := uuid.New()
		_, err := svc.RequestGeneration(context.Background(), actor, services.IdeaGenerationRequest{
			Count: 1, WindowDays: 7, ReportID: &reportID, ItemIDs: ids(1),
		})
		requireFieldError(t, err, "item_ids", "conflict")
	})

	t.Run("the window still applies", func(t *testing.T) {
		t.Parallel()
		svc, _ := newIdeaRequestService(t)
		_, err := svc.RequestGeneration(context.Background(), actor, services.IdeaGenerationRequest{Count: 1, WindowDays: 31, ItemIDs: ids(1)})
		requireFieldError(t, err, "window_days", "max")
	})

	t.Run("an unknown item is refused", func(t *testing.T) {
		t.Parallel()
		svc, d := newIdeaRequestService(t)
		picked := ids(2)
		d.prefs.EXPECT().EmailVerified(gomock.Any(), actor.UserID).Return(true, nil)
		d.candidates.EXPECT().ByIDs(gomock.Any(), picked).Return([]repositories.Candidate{{ItemID: picked[0]}}, nil)
		_, err := svc.RequestGeneration(context.Background(), actor, services.IdeaGenerationRequest{Count: 1, WindowDays: 7, ItemIDs: picked})
		requireFieldError(t, err, "item_ids", "not_found")
	})
}

func TestRequestGenerationEnqueuesPickedItems(t *testing.T) {
	t.Parallel()
	svc, d := newIdeaRequestService(t)
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}
	picked := []uuid.UUID{uuid.New(), uuid.New()}
	jobID := uuid.New()

	d.prefs.EXPECT().EmailVerified(gomock.Any(), actor.UserID).Return(true, nil)
	d.candidates.EXPECT().ByIDs(gomock.Any(), picked).Return([]repositories.Candidate{{ItemID: picked[0]}, {ItemID: picked[1]}}, nil)
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobGenerateIdeas, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, payload any, opts workers.EnqueueOptions) (uuid.UUID, error) {
			raw, err := json.Marshal(payload)
			require.NoError(t, err)
			var got struct {
				UserID     uuid.UUID   `json:"user_id"`
				Count      int         `json:"count"`
				WindowDays int         `json:"window_days"`
				ReportID   *uuid.UUID  `json:"report_id"`
				ItemIDs    []uuid.UUID `json:"item_ids"`
			}
			require.NoError(t, json.Unmarshal(raw, &got))
			assert.Equal(t, actor.UserID, got.UserID)
			assert.Equal(t, 2, got.Count)
			assert.Equal(t, 7, got.WindowDays)
			assert.Nil(t, got.ReportID)
			assert.Equal(t, picked, got.ItemIDs, "picked items keep their order")
			assert.Equal(t, "ideas:"+actor.UserID.String(), opts.DedupeKey)
			return jobID, nil
		})

	got, err := svc.RequestGeneration(context.Background(), actor, services.IdeaGenerationRequest{Count: 2, WindowDays: 7, ItemIDs: picked})
	require.NoError(t, err)
	assert.Equal(t, jobID, got)
}

func TestRequestGenerationNeedsVerifiedEmail(t *testing.T) {
	t.Parallel()
	svc, d := newIdeaRequestService(t)
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}

	d.prefs.EXPECT().EmailVerified(gomock.Any(), actor.UserID).Return(false, nil)
	_, err := svc.RequestGeneration(context.Background(), actor, services.IdeaGenerationRequest{Count: 1, WindowDays: 7})
	requireCode(t, err, "EMAIL_NOT_VERIFIED")
}
