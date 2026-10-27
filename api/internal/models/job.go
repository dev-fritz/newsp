package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JobStatus is the lifecycle state of a background job.
type JobStatus string

// Job statuses.
const (
	JobPending  JobStatus = "pending"
	JobRunning  JobStatus = "running"
	JobDone     JobStatus = "done"
	JobFailed   JobStatus = "failed"
	JobCanceled JobStatus = "canceled"
)

// Job is a unit of background work consumed by the worker.
type Job struct {
	ID          uuid.UUID
	Kind        string
	Payload     json.RawMessage
	UserID      *uuid.UUID
	Status      JobStatus
	Priority    int16
	RunAt       time.Time
	Attempts    int
	MaxAttempts int
	LastError   *string
	Result      json.RawMessage
	DedupeKey   *string
	LockedBy    *string
	LockedAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	FinishedAt  *time.Time
}
