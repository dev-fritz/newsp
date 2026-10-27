package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/bootstrap"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/tests/apitest"
)

func newMailServer(t *testing.T) (*apitest.Server, *apitest.Mailbox) {
	t.Helper()
	box := &apitest.Mailbox{}
	return apitest.NewWithOptions(t, bootstrap.Options{Mailer: box}), box
}

func TestEmailIsDeliveredAndErased(t *testing.T) {
	s, box := newMailServer(t)
	ctx := context.Background()
	email := services.Email{
		Kind: services.EmailPasswordReset, To: "ana@example.com", Locale: "en",
		Link: "https://newsp.test/reset-password/abcdefghijklmnopqrstuvwxyz", ValidFor: time.Hour,
	}
	require.NoError(t, s.Container.Tx.WithinTx(ctx, func(ctx context.Context) error {
		return s.Container.Emails.Enqueue(ctx, nil, email)
	}))

	var payload string
	require.NoError(t, s.Container.DB.QueryRowContext(ctx,
		`SELECT payload::text FROM jobs WHERE kind = $1`, services.JobSendEmail).Scan(&payload))
	assert.NotContains(t, payload, "abcdefghijklmnopqrstuvwxyz", "the link is sealed in the queue")

	runJobs(t, s)

	msgs := box.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "Reset your newsp password", msgs[0].Subject)
	assert.Equal(t, "abcdefghijklmnopqrstuvwxyz", box.LastToken("ana@example.com"))

	var status string
	require.NoError(t, s.Container.DB.QueryRowContext(ctx,
		`SELECT status, payload::text FROM jobs WHERE kind = $1`, services.JobSendEmail).Scan(&status, &payload))
	assert.Equal(t, "done", status)
	assert.Equal(t, "{}", payload)
}

func TestEmailIsNotQueuedWhenTheTransactionRollsBack(t *testing.T) {
	s, _ := newMailServer(t)
	ctx := context.Background()
	err := s.Container.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.Container.Emails.Enqueue(ctx, nil, services.Email{
			Kind: services.EmailInvite, To: "bia@example.com", Locale: "pt-BR", Link: "https://newsp.test/invite/x", ValidFor: time.Hour,
		}); err != nil {
			return err
		}
		return errors.New("boom")
	})
	require.Error(t, err)

	var n int
	require.NoError(t, s.Container.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM jobs WHERE kind = $1`, services.JobSendEmail).Scan(&n))
	assert.Zero(t, n)
}
