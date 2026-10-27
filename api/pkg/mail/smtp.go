package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// TLSMode selects how the connection to the SMTP server is secured.
type TLSMode string

// Supported TLS modes.
const (
	// TLSStartTLS connects in plain text and upgrades with STARTTLS
	// (usually port 587). The server must offer it; there is no fallback to
	// plain text. It is the default when the mode is empty.
	TLSStartTLS TLSMode = "starttls"
	// TLSImplicit wraps the connection in TLS from the first byte (usually
	// port 465).
	TLSImplicit TLSMode = "tls"
	// TLSNone sends everything in plain text. Use it only for a local relay
	// such as Mailpit.
	TLSNone TLSMode = "none"
)

// defaultTimeout bounds one Send when neither the context nor the config
// provides a deadline.
const defaultTimeout = 15 * time.Second

// SMTPConfig configures an SMTP sender.
type SMTPConfig struct {
	Host string
	Port int
	// Username and Password enable PLAIN authentication when Username is set.
	Username string
	Password string
	TLS      TLSMode
	// From is the sender, as "no-reply@example.com" or "newsp <no-reply@example.com>".
	From string
	// Timeout bounds the whole exchange; zero means 15 seconds. A shorter
	// context deadline wins.
	Timeout time.Duration
}

// SMTP sends messages through an SMTP server. It is safe for concurrent use:
// every Send opens its own connection.
type SMTP struct {
	cfg SMTPConfig
	// tlsConfig replaces the default TLS settings; tests use it to trust a
	// self-signed certificate.
	tlsConfig *tls.Config
}

var _ Mailer = (*SMTP)(nil)

// NewSMTP builds an SMTP sender. The configuration is validated on Send.
func NewSMTP(cfg SMTPConfig) *SMTP {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.TLS == "" {
		cfg.TLS = TLSStartTLS
	}
	return &SMTP{cfg: cfg}
}

// Send delivers msg. The context bounds dialing, the SMTP conversation and
// TLS negotiation; cancelling it aborts the send.
func (m *SMTP) Send(ctx context.Context, msg Message) error {
	if err := m.send(ctx, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

func (m *SMTP) send(ctx context.Context, msg Message) error {
	from, err := parseAddress("from", m.cfg.From)
	if err != nil {
		return err
	}
	to, err := parseAddress("to", msg.To)
	if err != nil {
		return err
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return errors.New("subject must not contain line breaks")
	}
	switch m.cfg.TLS {
	case TLSStartTLS, TLSImplicit, TLSNone:
	default:
		return fmt.Errorf("unknown TLS mode %q", m.cfg.TLS)
	}
	if m.cfg.Host == "" {
		return errors.New("host is required")
	}
	data, err := buildMessage(from, to, msg, time.Now())
	if err != nil {
		return err
	}

	deadline := time.Now().Add(m.cfg.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	conn, err := m.dial(ctx)
	if err != nil {
		return err
	}
	// Closing the connection is what unblocks reads and writes when the
	// context ends; the deadline covers a server that stops answering.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	if err := m.converse(conn, from.Address, to.Address, data); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("%w (%w)", err, ctxErr)
		}
		// The connection deadline can fire just before the context notices
		// the same instant; report it as the deadline it is.
		if !time.Now().Before(deadline) {
			return fmt.Errorf("%w (%w)", err, context.DeadlineExceeded)
		}
		return err
	}
	return nil
}

func (m *SMTP) dial(ctx context.Context) (net.Conn, error) {
	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	dialer := &net.Dialer{}
	if m.cfg.TLS == TLSImplicit {
		td := &tls.Dialer{NetDialer: dialer, Config: m.tlsClientConfig()}
		conn, err := td.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", addr, err)
		}
		return conn, nil
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return conn, nil
}

func (m *SMTP) tlsClientConfig() *tls.Config {
	cfg := &tls.Config{}
	if m.tlsConfig != nil {
		cfg = m.tlsConfig.Clone()
	}
	if cfg.ServerName == "" {
		cfg.ServerName = m.cfg.Host
	}
	if cfg.MinVersion == 0 {
		cfg.MinVersion = tls.VersionTLS12
	}
	return cfg
}

// converse runs the SMTP dialogue over an established connection.
func (m *SMTP) converse(conn net.Conn, from, to string, data []byte) error {
	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("greeting: %w", err)
	}
	defer func() { _ = client.Close() }()
	// net/smtp greets as "localhost" by default, which strict servers and spam
	// filters dislike; the sender's domain is a better HELO name.
	if err := client.Hello(heloName(from)); err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	if m.cfg.TLS == TLSStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("server does not offer STARTTLS; refusing to continue in plain text")
		}
		if err := client.StartTLS(m.tlsClientConfig()); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if m.cfg.Username != "" {
		// PlainAuth itself refuses to send the password over an unencrypted
		// connection to anything but localhost.
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("end data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit: %w", err)
	}
	return nil
}

// parseAddress rejects header injection and validates one mailbox.
func parseAddress(field, raw string) (*mail.Address, error) {
	if strings.ContainsAny(raw, "\r\n") {
		return nil, fmt.Errorf("%s address must not contain line breaks", field)
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s address: %w", field, err)
	}
	return addr, nil
}

// buildMessage renders the RFC 5322 message with CRLF line endings.
func buildMessage(from, to *mail.Address, msg Message, now time.Time) ([]byte, error) {
	id, err := messageID(from.Address)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", from.String())
	header("To", to.String())
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("Date", now.UTC().Format(time.RFC1123Z))
	header("Message-ID", id)
	header("MIME-Version", "1.0")

	if msg.HTML == "" {
		header("Content-Type", `text/plain; charset="UTF-8"`)
		header("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		if err := writeQP(&b, msg.Text); err != nil {
			return nil, err
		}
		return []byte(b.String()), nil
	}

	mw := multipart.NewWriter(&b)
	header("Content-Type", mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": mw.Boundary()}))
	b.WriteString("\r\n")
	for _, part := range []struct{ contentType, body string }{
		{`text/plain; charset="UTF-8"`, msg.Text},
		{`text/html; charset="UTF-8"`, msg.HTML},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.contentType},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, fmt.Errorf("build message: %w", err)
		}
		if err := writeQP(pw, part.body); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("build message: %w", err)
	}
	return []byte(b.String()), nil
}

func writeQP(w io.Writer, s string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(s)); err != nil {
		return fmt.Errorf("encode body: %w", err)
	}
	if err := qp.Close(); err != nil {
		return fmt.Errorf("encode body: %w", err)
	}
	return nil
}

// messageID returns a unique Message-ID under the sender's domain.
func messageID(sender string) (string, error) {
	domain := "localhost"
	if i := strings.LastIndex(sender, "@"); i >= 0 && i < len(sender)-1 {
		domain = sender[i+1:]
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("message id: %w", err)
	}
	return "<" + hex.EncodeToString(raw[:]) + "@" + domain + ">", nil
}

// heloName is the domain of the sender address, or "localhost" without one.
func heloName(from string) string {
	if i := strings.LastIndexByte(from, '@'); i >= 0 && i < len(from)-1 {
		return from[i+1:]
	}
	return "localhost"
}
