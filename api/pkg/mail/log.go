package mail

import (
	"context"
	"log/slog"
)

// Log is a Mailer that writes messages to the logger instead of sending them.
// It is meant for development and for installs without an SMTP server.
type Log struct {
	logger      *slog.Logger
	includeBody bool
}

var _ Mailer = (*Log)(nil)

// NewLog builds a log mailer. With includeBody the plain-text body is logged
// too, so a developer can copy verification links; leave it off elsewhere,
// since bodies carry single-use tokens.
func NewLog(logger *slog.Logger, includeBody bool) *Log {
	return &Log{logger: logger, includeBody: includeBody}
}

// Send logs the recipient and subject (and the body when enabled). It never
// fails.
func (l *Log) Send(ctx context.Context, msg Message) error {
	attrs := []any{"to", msg.To, "subject", msg.Subject}
	if l.includeBody {
		attrs = append(attrs, "body", msg.Text)
	}
	l.logger.InfoContext(ctx, "mail not sent (log mailer)", attrs...)
	return nil
}
