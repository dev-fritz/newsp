package services

//go:generate go tool mockgen -source=email_service.go -destination=../../tests/mocks/email_mocks.go -package=mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/mail"
)

// JobSendEmail delivers one transactional e-mail.
const JobSendEmail = "email.send"

// emailMaxAttempts covers a mail server that is down for about an hour.
const emailMaxAttempts = 8

// EmailKind selects the template of a transactional e-mail.
type EmailKind string

// Transactional e-mails.
const (
	EmailVerification  EmailKind = "email_verification"
	EmailPasswordReset EmailKind = "password_reset"
	EmailInvite        EmailKind = "invite"
)

// Email is a transactional message before rendering. Link carries a one-time
// token, so the whole value is sealed while it waits in the queue.
type Email struct {
	Kind     EmailKind     `json:"kind"`
	To       string        `json:"to"`
	Name     string        `json:"name,omitempty"`
	Locale   string        `json:"locale"`
	Link     string        `json:"link"`
	ValidFor time.Duration `json:"valid_for"`
}

// Mailer sends a rendered message.
type Mailer interface {
	Send(ctx context.Context, msg mail.Message) error
}

// EmailJobStore erases a delivered e-mail from its job.
type EmailJobStore interface {
	ClearPayload(ctx context.Context, id uuid.UUID) error
}

// emailPayload is the job payload: the sealed JSON of an Email.
type emailPayload struct {
	Sealed []byte `json:"sealed"`
}

// EmailService queues transactional e-mails and delivers them from the worker.
// Queuing inside the caller's transaction means a message goes out only if the
// change that triggered it is committed.
type EmailService struct {
	queue  Enqueuer
	box    SecretBox
	mailer Mailer
	jobs   EmailJobStore
	ready  bool
}

// NewEmailService builds an EmailService. ready reports whether e-mails reach
// people (a mail server is configured, or the app runs in development, where
// the log mailer prints them).
func NewEmailService(queue Enqueuer, box SecretBox, mailer Mailer, jobs EmailJobStore, ready bool) *EmailService {
	return &EmailService{queue: queue, box: box, mailer: mailer, jobs: jobs, ready: ready}
}

// Ready reports whether e-mails reach people. Features that only work through
// e-mail (public signup) require it.
func (s *EmailService) Ready() bool { return s.ready }

// Enqueue queues e for delivery. Call it inside the transaction of the change
// that triggers the e-mail.
func (s *EmailService) Enqueue(ctx context.Context, userID *uuid.UUID, e Email) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encode email: %w", err)
	}
	sealed, err := s.box.Seal(raw)
	if err != nil {
		return fmt.Errorf("seal email: %w", err)
	}
	_, err = s.queue.Enqueue(ctx, JobSendEmail, emailPayload{Sealed: sealed},
		workers.EnqueueOptions{UserID: userID, Priority: 10, MaxAttempts: emailMaxAttempts})
	if err != nil {
		return fmt.Errorf("enqueue email: %w", err)
	}
	return nil
}

// Deliver is the JobSendEmail handler. After a successful send the payload is
// erased, so the link does not outlive its delivery even in sealed form.
func (s *EmailService) Deliver(ctx context.Context, job models.Job) (any, error) {
	var p emailPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, workers.Permanent(fmt.Errorf("decode email payload: %w", err))
	}
	if len(p.Sealed) == 0 {
		// Delivered by an earlier attempt whose completion was not recorded.
		return map[string]string{"skipped": "already delivered"}, nil
	}
	raw, err := s.box.Open(p.Sealed)
	if err != nil {
		return nil, workers.Permanent(fmt.Errorf("open email payload: %w", err))
	}
	var e Email
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, workers.Permanent(fmt.Errorf("decode email: %w", err))
	}
	msg, err := renderEmail(e)
	if err != nil {
		return nil, workers.Permanent(err)
	}
	if err := s.mailer.Send(ctx, msg); err != nil {
		return nil, err
	}
	if err := s.jobs.ClearPayload(ctx, job.ID); err != nil {
		// The message is out; retrying would send it twice. The sealed payload
		// stays until finished jobs are purged.
		return map[string]string{"kind": string(e.Kind), "payload": "kept: " + err.Error()}, nil //nolint:nilerr // see above
	}
	return map[string]string{"kind": string(e.Kind)}, nil
}
