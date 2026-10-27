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

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/mail"
	"github.com/dev-fritz/newsp/api/pkg/secretbox"
	"github.com/dev-fritz/newsp/api/tests/mocks"
)

type emailDeps struct {
	queue  *mocks.MockEnqueuer
	mailer *mocks.MockMailer
	jobs   *mocks.MockEmailJobStore
	box    *secretbox.Box
}

func newEmailService(t *testing.T) (*services.EmailService, emailDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	d := emailDeps{
		queue: mocks.NewMockEnqueuer(ctrl), mailer: mocks.NewMockMailer(ctrl),
		jobs: mocks.NewMockEmailJobStore(ctrl), box: box,
	}
	return services.NewEmailService(d.queue, box, d.mailer, d.jobs, true), d
}

// queuedJob enqueues e and returns the job the queue received.
func queuedJob(t *testing.T, svc *services.EmailService, d emailDeps, e services.Email) models.Job {
	t.Helper()
	userID := uuid.New()
	var payload []byte
	d.queue.EXPECT().Enqueue(gomock.Any(), services.JobSendEmail, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, p any, opts workers.EnqueueOptions) (uuid.UUID, error) {
			assert.Equal(t, &userID, opts.UserID)
			assert.Equal(t, 8, opts.MaxAttempts)
			var err error
			payload, err = json.Marshal(p)
			require.NoError(t, err)
			return uuid.New(), nil
		})
	require.NoError(t, svc.Enqueue(context.Background(), &userID, e))
	return models.Job{ID: uuid.New(), Kind: services.JobSendEmail, Payload: payload}
}

func verificationEmail() services.Email {
	return services.Email{
		Kind: services.EmailVerification, To: "ana@example.com", Name: "Ana", Locale: "pt-BR",
		Link: "https://newsp.test/verify-email/tok123", ValidFor: 24 * time.Hour,
	}
}

func TestEmailEnqueueSealsTheLink(t *testing.T) {
	t.Parallel()
	svc, d := newEmailService(t)
	job := queuedJob(t, svc, d, verificationEmail())

	assert.NotContains(t, string(job.Payload), "tok123")
	assert.NotContains(t, string(job.Payload), "ana@example.com")
}

func TestEmailDeliverSendsAndClearsPayload(t *testing.T) {
	t.Parallel()
	svc, d := newEmailService(t)
	job := queuedJob(t, svc, d, verificationEmail())

	var sent mail.Message
	d.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m mail.Message) error {
		sent = m
		return nil
	})
	d.jobs.EXPECT().ClearPayload(gomock.Any(), job.ID).Return(nil)

	res, err := svc.Deliver(context.Background(), job)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"kind": "email_verification"}, res)
	assert.Equal(t, "ana@example.com", sent.To)
	assert.Equal(t, "Confirme seu e-mail no newsp", sent.Subject)
	assert.Contains(t, sent.Text, "Olá, Ana!")
	assert.Contains(t, sent.Text, "https://newsp.test/verify-email/tok123")
	assert.Contains(t, sent.Text, "24 horas")
	assert.Contains(t, sent.HTML, `href="https://newsp.test/verify-email/tok123"`)
	assert.Contains(t, sent.HTML, `lang="pt-BR"`)
}

func TestEmailDeliverRendersEnglishAndFallsBack(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		locale  string
		kind    services.EmailKind
		valid   time.Duration
		subject string
		expiry  string
	}{
		"english reset":       {"en", services.EmailPasswordReset, time.Hour, "Reset your newsp password", "1 hour"},
		"unknown locale":      {"fr", services.EmailInvite, 7 * 24 * time.Hour, "You are invited to newsp", "7 days"},
		"portuguese invite":   {"pt-BR", services.EmailInvite, 7 * 24 * time.Hour, "Você recebeu um convite para o newsp", "7 dias"},
		"portuguese in hours": {"pt-BR", services.EmailPasswordReset, 2 * time.Hour, "Redefina sua senha do newsp", "2 horas"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, d := newEmailService(t)
			e := verificationEmail()
			e.Locale, e.Kind, e.ValidFor, e.Name = tc.locale, tc.kind, tc.valid, ""
			job := queuedJob(t, svc, d, e)

			d.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m mail.Message) error {
				assert.Equal(t, tc.subject, m.Subject)
				assert.Contains(t, m.Text, tc.expiry)
				return nil
			})
			d.jobs.EXPECT().ClearPayload(gomock.Any(), job.ID).Return(nil)
			_, err := svc.Deliver(context.Background(), job)
			require.NoError(t, err)
		})
	}
}

func TestEmailDeliverEscapesTheNameInHTML(t *testing.T) {
	t.Parallel()
	svc, d := newEmailService(t)
	e := verificationEmail()
	e.Name = `<script>alert(1)</script>`
	job := queuedJob(t, svc, d, e)

	d.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m mail.Message) error {
		assert.NotContains(t, m.HTML, "<script>")
		assert.Contains(t, m.HTML, "&lt;script&gt;")
		return nil
	})
	d.jobs.EXPECT().ClearPayload(gomock.Any(), job.ID).Return(nil)
	_, err := svc.Deliver(context.Background(), job)
	require.NoError(t, err)
}

func TestEmailDeliverRetriesWhenTheServerFails(t *testing.T) {
	t.Parallel()
	svc, d := newEmailService(t)
	job := queuedJob(t, svc, d, verificationEmail())
	d.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(errors.New("connection refused"))

	_, err := svc.Deliver(context.Background(), job)
	require.Error(t, err)
	assert.False(t, workers.IsPermanent(err), "a send failure must be retried")
}

func TestEmailDeliverKeepsSuccessWhenClearingFails(t *testing.T) {
	t.Parallel()
	svc, d := newEmailService(t)
	job := queuedJob(t, svc, d, verificationEmail())
	d.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(nil)
	d.jobs.EXPECT().ClearPayload(gomock.Any(), job.ID).Return(errors.New("db down"))

	res, err := svc.Deliver(context.Background(), job)
	require.NoError(t, err, "the e-mail is out; a retry would send it twice")
	assert.Equal(t, "kept: db down", res.(map[string]string)["payload"])
}

func TestEmailDeliverSkipsAClearedPayload(t *testing.T) {
	t.Parallel()
	svc, _ := newEmailService(t)
	res, err := svc.Deliver(context.Background(), models.Job{ID: uuid.New(), Payload: []byte(`{}`)})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"skipped": "already delivered"}, res)
}

func TestEmailDeliverRejectsBadPayloads(t *testing.T) {
	t.Parallel()
	cases := map[string][]byte{
		"not json":     []byte(`nope`),
		"wrong key":    []byte(`{"sealed":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`),
		"unknown kind": nil, // filled below
		"missing link": nil,
	}
	svc, d := newEmailService(t)
	seal := func(e services.Email) []byte {
		raw, err := json.Marshal(e)
		require.NoError(t, err)
		sealed, err := d.box.Seal(raw)
		require.NoError(t, err)
		p, err := json.Marshal(map[string][]byte{"sealed": sealed})
		require.NoError(t, err)
		return p
	}
	unknown := verificationEmail()
	unknown.Kind = "newsletter"
	cases["unknown kind"] = seal(unknown)
	noLink := verificationEmail()
	noLink.Link = ""
	cases["missing link"] = seal(noLink)

	for name, payload := range cases {
		_, err := svc.Deliver(context.Background(), models.Job{ID: uuid.New(), Payload: payload})
		require.Error(t, err, name)
		assert.True(t, workers.IsPermanent(err), "%s: bad payloads must not be retried", name)
	}
}
