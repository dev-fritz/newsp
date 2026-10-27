package mail

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	netmail "net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// received is what the fake server captured from one SMTP transaction.
type received struct {
	from, to string
	auth     string // decoded AUTH PLAIN payload
	data     []byte
	secure   bool // the session was upgraded with STARTTLS or started in TLS
}

// fakeServer speaks just enough SMTP to receive one message at a time.
type fakeServer struct {
	t        *testing.T
	ln       net.Listener
	tlsCfg   *tls.Config // certificate for implicit TLS and STARTTLS
	implicit bool        // wrap accepted connections in TLS
	starttls bool        // advertise and accept STARTTLS
	auth     bool        // advertise AUTH PLAIN

	mu   sync.Mutex
	msgs []received
	helo string // argument of the last EHLO
}

func newFakeServer(t *testing.T, configure func(*fakeServer)) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &fakeServer{t: t, ln: ln}
	if configure != nil {
		configure(s)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go s.serve()
	return s
}

func (s *fakeServer) port() int {
	return s.ln.Addr().(*net.TCPAddr).Port
}

func (s *fakeServer) messages() []received {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]received(nil), s.msgs...)
}

func (s *fakeServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	secure := false
	if s.implicit {
		conn = tls.Server(conn, s.tlsCfg)
		secure = true
	}
	r := textproto.NewReader(bufio.NewReader(conn))
	reply := func(format string) { _, _ = io.WriteString(conn, format+"\r\n") }

	var cur received
	reply("220 fake ESMTP")
	for {
		line, err := r.ReadLine()
		if err != nil {
			return
		}
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			s.mu.Lock()
			s.helo = strings.TrimSpace(line[len("EHLO"):])
			s.mu.Unlock()
			_, _ = io.WriteString(conn, "250-fake\r\n")
			if s.starttls && !secure {
				_, _ = io.WriteString(conn, "250-STARTTLS\r\n")
			}
			if s.auth {
				_, _ = io.WriteString(conn, "250-AUTH PLAIN\r\n")
			}
			reply("250 8BITMIME")
		case cmd == "STARTTLS":
			reply("220 go ahead")
			conn = tls.Server(conn, s.tlsCfg)
			r = textproto.NewReader(bufio.NewReader(conn))
			secure = true
		case strings.HasPrefix(cmd, "AUTH PLAIN "):
			payload, err := base64.StdEncoding.DecodeString(line[len("AUTH PLAIN "):])
			if err != nil {
				reply("501 bad payload")
				continue
			}
			cur.auth = strings.ReplaceAll(string(payload), "\x00", "|")
			reply("235 ok")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			cur.from = mailbox(line[len("MAIL FROM:"):])
			reply("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			cur.to = mailbox(line[len("RCPT TO:"):])
			reply("250 ok")
		case cmd == "DATA":
			reply("354 end with .")
			body, err := r.ReadDotBytes()
			if err != nil {
				return
			}
			cur.data = body
			cur.secure = secure
			s.mu.Lock()
			s.msgs = append(s.msgs, cur)
			s.mu.Unlock()
			cur = received{}
			reply("250 queued")
		case cmd == "QUIT":
			reply("221 bye")
			return
		default:
			reply("502 not implemented")
		}
	}
}

// mailbox extracts the address from "<addr> PARAMS".
func mailbox(arg string) string {
	arg = strings.TrimSpace(arg)
	if i := strings.Index(arg, ">"); i >= 0 {
		arg = arg[:i]
	}
	return strings.TrimPrefix(arg, "<")
}

// selfSigned returns a server config and a client config that trusts it.
func selfSigned(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{
			Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
			MinVersion:   tls.VersionTLS12,
		},
		&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
}

func (s *fakeServer) config(mode TLSMode) SMTPConfig {
	return SMTPConfig{
		Host:    "127.0.0.1",
		Port:    s.port(),
		TLS:     mode,
		From:    "newsp <no-reply@example.com>",
		Timeout: 5 * time.Second,
	}
}

// parse splits a captured message into its headers and body.
func parse(t *testing.T, data []byte) *netmail.Message {
	t.Helper()
	m, err := netmail.ReadMessage(bytes.NewReader(data))
	require.NoError(t, err)
	return m
}

func TestSMTPSendPlainText(t *testing.T) {
	t.Parallel()
	srv := newFakeServer(t, func(s *fakeServer) { s.auth = true })

	cfg := srv.config(TLSNone)
	cfg.Username, cfg.Password = "resend", "s3cret"
	err := NewSMTP(cfg).Send(t.Context(), Message{
		To:      "Ana <ana@example.org>",
		Subject: "Confirme seu e-mail — olá",
		Text:    "Olá, confirme: https://app.example.com/verify?token=abc\n",
	})
	require.NoError(t, err)

	got := srv.messages()
	require.Len(t, got, 1)
	assert.Equal(t, "no-reply@example.com", got[0].from)
	assert.Equal(t, "ana@example.org", got[0].to)
	assert.Equal(t, "|resend|s3cret", got[0].auth)
	srv.mu.Lock()
	assert.Equal(t, "example.com", srv.helo, "HELO uses the sender's domain")
	srv.mu.Unlock()

	m := parse(t, got[0].data)
	assert.Equal(t, "1.0", m.Header.Get("MIME-Version"))
	assert.Equal(t, `"newsp" <no-reply@example.com>`, m.Header.Get("From"))
	assert.Equal(t, `"Ana" <ana@example.org>`, m.Header.Get("To"))
	assert.Regexp(t, `^<[0-9a-f]{32}@example\.com>$`, m.Header.Get("Message-ID"))
	_, err = m.Header.Date()
	require.NoError(t, err)

	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	require.NoError(t, err)
	assert.Equal(t, "Confirme seu e-mail — olá", subject)
	assert.NotContains(t, m.Header.Get("Subject"), "—", "non-ASCII subject must be encoded")

	mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, "text/plain", mediaType)
	assert.Equal(t, "UTF-8", params["charset"])
	assert.Equal(t, "quoted-printable", m.Header.Get("Content-Transfer-Encoding"))
	body, err := io.ReadAll(quotedprintable.NewReader(m.Body))
	require.NoError(t, err)
	assert.Equal(t, "Olá, confirme: https://app.example.com/verify?token=abc\n", strings.ReplaceAll(string(body), "\r\n", "\n"))
}

func TestSMTPSendMultipart(t *testing.T) {
	t.Parallel()
	srv := newFakeServer(t, nil)

	err := NewSMTP(srv.config(TLSNone)).Send(t.Context(), Message{
		To:      "ana@example.org",
		Subject: "Hello",
		Text:    "texto: açúcar",
		HTML:    `<p>html: <a href="https://x.test/?a=1&b=2">açúcar</a></p>`,
	})
	require.NoError(t, err)

	got := srv.messages()
	require.Len(t, got, 1)
	m := parse(t, got[0].data)
	assert.Equal(t, "Hello", m.Header.Get("Subject"), "ASCII subject stays as is")

	mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, "multipart/alternative", mediaType)

	mr := multipart.NewReader(m.Body, params["boundary"])
	var types, bodies []string
	for {
		p, err := mr.NextPart() // decodes quoted-printable transparently
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		types = append(types, strings.SplitN(p.Header.Get("Content-Type"), ";", 2)[0])
		b, err := io.ReadAll(p)
		require.NoError(t, err)
		bodies = append(bodies, string(b))
	}
	assert.Equal(t, []string{"text/plain", "text/html"}, types)
	assert.Equal(t, []string{
		"texto: açúcar",
		`<p>html: <a href="https://x.test/?a=1&b=2">açúcar</a></p>`,
	}, bodies)
}

func TestSMTPSendLongLinesAreWrapped(t *testing.T) {
	t.Parallel()
	srv := newFakeServer(t, nil)

	text := "https://app.example.com/verify?token=" + strings.Repeat("a", 300)
	require.NoError(t, NewSMTP(srv.config(TLSNone)).Send(t.Context(), Message{
		To: "ana@example.org", Subject: "Hi", Text: text,
	}))

	m := parse(t, srv.messages()[0].data)
	body, err := io.ReadAll(quotedprintable.NewReader(m.Body))
	require.NoError(t, err)
	assert.Equal(t, text, strings.TrimSpace(string(body)))
}

func TestSMTPSendRejectsUnsafeInput(t *testing.T) {
	t.Parallel()
	srv := newFakeServer(t, nil)

	tests := []struct {
		name    string
		mutate  func(*SMTPConfig, *Message)
		wantErr string
	}{
		{"CRLF in recipient", func(_ *SMTPConfig, m *Message) { m.To = "a@example.org\r\nBcc: evil@example.net" }, "line breaks"},
		{"LF in recipient", func(_ *SMTPConfig, m *Message) { m.To = "a@example.org\nBcc: evil@example.net" }, "line breaks"},
		{"CRLF in subject", func(_ *SMTPConfig, m *Message) { m.Subject = "hi\r\nBcc: evil@example.net" }, "line breaks"},
		{"LF in subject", func(_ *SMTPConfig, m *Message) { m.Subject = "hi\nBcc: evil@example.net" }, "line breaks"},
		{"CRLF in from", func(c *SMTPConfig, _ *Message) { c.From = "a@example.com\r\nBcc: evil@example.net" }, "line breaks"},
		{"invalid recipient", func(_ *SMTPConfig, m *Message) { m.To = "not-an-address" }, "invalid to address"},
		{"empty recipient", func(_ *SMTPConfig, m *Message) { m.To = "" }, "invalid to address"},
		{"invalid from", func(c *SMTPConfig, _ *Message) { c.From = "nope" }, "invalid from address"},
		{"unknown tls mode", func(c *SMTPConfig, _ *Message) { c.TLS = "ssl" }, "unknown TLS mode"},
		{"missing host", func(c *SMTPConfig, _ *Message) { c.Host = "" }, "host is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := srv.config(TLSNone)
			msg := Message{To: "ana@example.org", Subject: "hi", Text: "x"}
			tt.mutate(&cfg, &msg)

			err := NewSMTP(cfg).Send(t.Context(), msg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
	assert.Empty(t, srv.messages(), "nothing may reach the server")
}

func TestSMTPSendStartTLS(t *testing.T) {
	t.Parallel()
	serverTLS, clientTLS := selfSigned(t)
	srv := newFakeServer(t, func(s *fakeServer) {
		s.tlsCfg, s.starttls, s.auth = serverTLS, true, true
	})

	cfg := srv.config(TLSStartTLS)
	cfg.Username, cfg.Password = "user", "pw"
	m := NewSMTP(cfg)
	m.tlsConfig = clientTLS
	require.NoError(t, m.Send(t.Context(), Message{To: "ana@example.org", Subject: "hi", Text: "x"}))

	got := srv.messages()
	require.Len(t, got, 1)
	assert.True(t, got[0].secure)
	assert.Equal(t, "|user|pw", got[0].auth)
}

func TestSMTPStartTLSIsRequired(t *testing.T) {
	t.Parallel()
	srv := newFakeServer(t, func(s *fakeServer) { s.auth = true }) // does not advertise STARTTLS

	cfg := srv.config(TLSStartTLS)
	cfg.Username, cfg.Password = "user", "super-secret-password"
	err := NewSMTP(cfg).Send(t.Context(), Message{To: "ana@example.org", Subject: "hi", Text: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STARTTLS")
	assert.NotContains(t, err.Error(), "super-secret-password")
	assert.Empty(t, srv.messages(), "must not fall back to plain text")
}

func TestSMTPStartTLSIsTheDefaultMode(t *testing.T) {
	t.Parallel()
	srv := newFakeServer(t, nil)

	cfg := srv.config("")
	err := NewSMTP(cfg).Send(t.Context(), Message{To: "ana@example.org", Subject: "hi", Text: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STARTTLS")
}

func TestSMTPStartTLSRejectsUntrustedCertificate(t *testing.T) {
	t.Parallel()
	serverTLS, _ := selfSigned(t)
	srv := newFakeServer(t, func(s *fakeServer) { s.tlsCfg, s.starttls = serverTLS, true })

	// Default TLS settings: the self-signed certificate is not trusted.
	err := NewSMTP(srv.config(TLSStartTLS)).Send(t.Context(), Message{To: "ana@example.org", Subject: "hi", Text: "x"})
	require.Error(t, err)
	assert.Empty(t, srv.messages())
}

func TestSMTPSendImplicitTLS(t *testing.T) {
	t.Parallel()
	serverTLS, clientTLS := selfSigned(t)
	srv := newFakeServer(t, func(s *fakeServer) { s.tlsCfg, s.implicit = serverTLS, true })

	m := NewSMTP(srv.config(TLSImplicit))
	m.tlsConfig = clientTLS
	require.NoError(t, m.Send(t.Context(), Message{To: "ana@example.org", Subject: "hi", Text: "x"}))

	got := srv.messages()
	require.Len(t, got, 1)
	assert.True(t, got[0].secure)
}

func TestSMTPContext(t *testing.T) {
	t.Parallel()

	// A server that accepts the connection and never greets.
	silent := func(t *testing.T) SMTPConfig {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				t.Cleanup(func() { _ = c.Close() })
			}
		}()
		return SMTPConfig{
			Host: "127.0.0.1",
			Port: ln.Addr().(*net.TCPAddr).Port,
			TLS:  TLSNone,
			From: "no-reply@example.com",
		}
	}
	msg := Message{To: "ana@example.org", Subject: "hi", Text: "x"}

	t.Run("already cancelled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := NewSMTP(silent(t)).Send(ctx, msg)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("cancelled while waiting for the server", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(100*time.Millisecond, cancel)
		start := time.Now()
		err := NewSMTP(silent(t)).Send(ctx, msg)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, time.Since(start), 3*time.Second)
	})

	t.Run("context deadline", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		err := NewSMTP(silent(t)).Send(ctx, msg)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("config timeout", func(t *testing.T) {
		t.Parallel()
		cfg := silent(t)
		cfg.Timeout = 100 * time.Millisecond
		start := time.Now()
		err := NewSMTP(cfg).Send(t.Context(), msg)
		require.Error(t, err)
		assert.Less(t, time.Since(start), 3*time.Second)
	})
}

func TestSMTPDialFailure(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	cfg := SMTPConfig{Host: "127.0.0.1", Port: port, TLS: TLSNone, From: "no-reply@example.com"}
	err = NewSMTP(cfg).Send(t.Context(), Message{To: "ana@example.org", Subject: "hi", Text: "x"})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "smtp send: "))
	assert.Contains(t, err.Error(), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
}

func TestNewSMTPDefaults(t *testing.T) {
	t.Parallel()
	m := NewSMTP(SMTPConfig{})
	assert.Equal(t, 15*time.Second, m.cfg.Timeout)
	assert.Equal(t, TLSStartTLS, m.cfg.TLS)
}
