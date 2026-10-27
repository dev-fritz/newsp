package dtos

import (
	"encoding/json"
	"time"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// JobResponse is the status of a background job, for polling.
type JobResponse struct {
	ID          string           `json:"id"`
	Kind        string           `json:"kind" example:"report.generate"`
	Status      models.JobStatus `json:"status" enums:"pending,running,done,failed,canceled"`
	Attempts    int              `json:"attempts"`
	MaxAttempts int              `json:"max_attempts"`
	Error       *string          `json:"error,omitempty"`
	Result      json.RawMessage  `json:"result,omitempty" swaggertype:"object"`
	RunAt       time.Time        `json:"run_at"`
	CreatedAt   time.Time        `json:"created_at"`
	FinishedAt  *time.Time       `json:"finished_at"`
}

// NewJobResponse maps a job; the raw error is included only when allowed.
func NewJobResponse(j models.Job, showError bool) JobResponse {
	resp := JobResponse{
		ID: j.ID.String(), Kind: j.Kind, Status: j.Status, Attempts: j.Attempts, MaxAttempts: j.MaxAttempts,
		Result: j.Result, RunAt: j.RunAt, CreatedAt: j.CreatedAt, FinishedAt: j.FinishedAt,
	}
	if showError {
		resp.Error = j.LastError
	}
	return resp
}

// JobEnvelope wraps JobResponse.
type JobEnvelope struct {
	Data JobResponse `json:"data"`
}

// JobAccepted is returned (202) by endpoints that enqueue work.
type JobAccepted struct {
	JobID string `json:"job_id"`
}

// JobAcceptedEnvelope wraps JobAccepted.
type JobAcceptedEnvelope struct {
	Data JobAccepted `json:"data"`
}
