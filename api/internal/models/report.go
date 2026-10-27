package models

import (
	"time"

	"github.com/google/uuid"
)

// ReportKind tells whether a report was scheduled or requested.
type ReportKind string

// Report kinds.
const (
	ReportScheduled ReportKind = "scheduled"
	ReportManual    ReportKind = "manual"
)

// ReportStatus tracks generation progress.
type ReportStatus string

// Report statuses.
const (
	ReportPending ReportStatus = "pending"
	ReportRunning ReportStatus = "running"
	ReportDone    ReportStatus = "done"
	ReportFailed  ReportStatus = "failed"
)

// Report is an immutable market report for one user and time window.
type Report struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Kind            ReportKind
	Status          ReportStatus
	WindowStart     time.Time
	WindowEnd       time.Time
	Focus           *string
	Title           *string
	Summary         *string
	ContentMD       *string
	OutputLang      string
	ModelID         *uuid.UUID
	ModelName       *string
	ItemsConsidered int
	Error           *string
	CreatedAt       time.Time
	CompletedAt     *time.Time
}

// ReportCitation links a [Ref] in a report to the cited item.
type ReportCitation struct {
	ReportID uuid.UUID
	ItemID   uuid.UUID
	Ref      int
}
