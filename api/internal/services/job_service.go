package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

// JobReader loads jobs.
type JobReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (models.Job, error)
}

// JobView is a job as its viewer may see it.
type JobView struct {
	Job models.Job
	// ShowError is true when the viewer may read the raw error (admins on
	// system jobs). Users only see the status of their own jobs.
	ShowError bool
}

// JobService exposes job status for polling.
type JobService struct {
	jobs JobReader
}

// NewJobService builds a JobService.
func NewJobService(jobs JobReader) *JobService {
	return &JobService{jobs: jobs}
}

// Get returns a job the actor owns, or a system job for admins. Anything else
// is reported as not found so job IDs of other users cannot be probed.
func (s *JobService) Get(ctx context.Context, actor auth.Identity, id uuid.UUID) (JobView, error) {
	notFound := apperrors.NotFound("JOB_NOT_FOUND", "job not found")
	job, err := s.jobs.GetByID(ctx, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return JobView{}, notFound
	}
	if err != nil {
		return JobView{}, fmt.Errorf("get job: %w", err)
	}
	switch {
	case job.UserID != nil && *job.UserID == actor.UserID:
		return JobView{Job: job, ShowError: auth.Can(actor, auth.PermJobsViewSystem)}, nil
	case job.UserID == nil && auth.Can(actor, auth.PermJobsViewSystem):
		return JobView{Job: job, ShowError: true}, nil
	default:
		return JobView{}, notFound
	}
}
