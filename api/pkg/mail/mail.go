// Package mail sends transactional e-mail (verification links, password
// resets) without tying the project to a vendor.
//
// SMTP is the only transport: it works with any provider and with a
// self-hosted server, and it needs nothing beyond the standard library.
// Resend, for example, is reachable as plain SMTP:
//
//	host     smtp.resend.com
//	port     465 (TLS "tls") or 587 (TLS "starttls")
//	username resend
//	password the Resend API key
//
// Amazon SES, Postmark, Mailpit and a local Postfix work the same way.
//
// Use [NewSMTP] in production and [NewLog] in development or when no SMTP
// server is configured: it writes the message to the logger instead of sending
// it. Both implement [Mailer].
package mail

import "context"

// Message is one e-mail for a single recipient.
type Message struct {
	// To is the recipient, as "user@example.com" or "Name <user@example.com>".
	To      string
	Subject string
	// Text is the plain-text body. It is always sent.
	Text string
	// HTML is an optional HTML alternative to Text.
	HTML string
}

// Mailer delivers a Message.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
