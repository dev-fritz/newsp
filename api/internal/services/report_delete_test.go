package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

func TestDeleteReport(t *testing.T) {
	t.Parallel()
	actor := auth.Identity{UserID: uuid.New(), Role: models.RoleMember}
	id := uuid.New()
	withStatus := func(s models.ReportStatus) models.Report {
		return models.Report{ID: id, UserID: actor.UserID, Status: s}
	}

	for _, status := range []models.ReportStatus{models.ReportDone, models.ReportFailed} {
		t.Run("owner deletes "+string(status), func(t *testing.T) {
			t.Parallel()
			svc, d := newReportService(t)
			d.reports.EXPECT().GetForUser(gomock.Any(), id, actor.UserID).Return(withStatus(status), nil)
			d.reports.EXPECT().DeleteForUser(gomock.Any(), id, actor.UserID).Return(nil)
			require.NoError(t, svc.Delete(context.Background(), actor, id))
		})
	}

	t.Run("missing or foreign report is not found", func(t *testing.T) {
		t.Parallel()
		svc, d := newReportService(t)
		d.reports.EXPECT().GetForUser(gomock.Any(), id, actor.UserID).Return(models.Report{}, repositories.ErrNotFound)
		requireCode(t, svc.Delete(context.Background(), actor, id), "REPORT_NOT_FOUND")
	})

	for _, status := range []models.ReportStatus{models.ReportPending, models.ReportRunning} {
		t.Run(string(status)+" is in progress", func(t *testing.T) {
			t.Parallel()
			svc, d := newReportService(t)
			d.reports.EXPECT().GetForUser(gomock.Any(), id, actor.UserID).Return(withStatus(status), nil)
			requireCode(t, svc.Delete(context.Background(), actor, id), "REPORT_IN_PROGRESS")
		})
	}

	t.Run("status race becomes in progress", func(t *testing.T) {
		t.Parallel()
		svc, d := newReportService(t)
		gomock.InOrder(
			d.reports.EXPECT().GetForUser(gomock.Any(), id, actor.UserID).Return(withStatus(models.ReportDone), nil),
			d.reports.EXPECT().DeleteForUser(gomock.Any(), id, actor.UserID).Return(repositories.ErrNotFound),
			d.reports.EXPECT().GetForUser(gomock.Any(), id, actor.UserID).Return(withStatus(models.ReportRunning), nil),
		)
		requireCode(t, svc.Delete(context.Background(), actor, id), "REPORT_IN_PROGRESS")
	})

	t.Run("deleted meanwhile is not found", func(t *testing.T) {
		t.Parallel()
		svc, d := newReportService(t)
		gomock.InOrder(
			d.reports.EXPECT().GetForUser(gomock.Any(), id, actor.UserID).Return(withStatus(models.ReportDone), nil),
			d.reports.EXPECT().DeleteForUser(gomock.Any(), id, actor.UserID).Return(repositories.ErrNotFound),
			d.reports.EXPECT().GetForUser(gomock.Any(), id, actor.UserID).Return(models.Report{}, repositories.ErrNotFound),
		)
		requireCode(t, svc.Delete(context.Background(), actor, id), "REPORT_NOT_FOUND")
	})

	t.Run("unexpected store error is propagated", func(t *testing.T) {
		t.Parallel()
		svc, d := newReportService(t)
		boom := errors.New("boom")
		d.reports.EXPECT().GetForUser(gomock.Any(), id, actor.UserID).Return(withStatus(models.ReportDone), nil)
		d.reports.EXPECT().DeleteForUser(gomock.Any(), id, actor.UserID).Return(boom)
		require.ErrorIs(t, svc.Delete(context.Background(), actor, id), boom)
	})
}
