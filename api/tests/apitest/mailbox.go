package apitest

import (
	"context"
	"regexp"
	"sync"

	"github.com/dev-fritz/newsp/api/pkg/mail"
)

// Mailbox is a fake mailer that keeps every message it is asked to send.
type Mailbox struct {
	mu   sync.Mutex
	msgs []mail.Message
}

// Send stores msg.
func (m *Mailbox) Send(_ context.Context, msg mail.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, msg)
	return nil
}

// Messages returns a copy of the messages sent so far.
func (m *Mailbox) Messages() []mail.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mail.Message(nil), m.msgs...)
}

var linkToken = regexp.MustCompile(`https?://\S+/([A-Za-z0-9_-]{20,})`)

// LastToken returns the one-time token of the last link sent to address, or
// "" when there is none.
func (m *Mailbox) LastToken(address string) string {
	msgs := m.Messages()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].To != address {
			continue
		}
		if match := linkToken.FindStringSubmatch(msgs[i].Text); match != nil {
			return match[1]
		}
	}
	return ""
}
