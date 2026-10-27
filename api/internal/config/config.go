// Package config loads and validates the application configuration from the
// environment. It is the only package allowed to read environment variables.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	nmail "net/mail"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment names accepted in APP_ENV.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
	EnvTest        = "test"
)

// Config holds every setting the processes need.
type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string
	// DatabaseAllowPlaintext accepts sslmode=disable in production, for a
	// database reachable only on a private network (docker compose).
	DatabaseAllowPlaintext bool
	LogLevel               slog.Level
	LogFormat              string
	CORSOrigins            []string
	ShutdownTimeout        time.Duration
	SwaggerEnabled         bool

	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	// AppBaseURL is the public URL of the web app, used to build invite and
	// password reset links.
	AppBaseURL string
	// TrustedProxies are the networks allowed to set X-Forwarded-For (the BFF).
	TrustedProxies []netip.Prefix
	// EncryptionKey (32 bytes) encrypts provider API keys at rest.
	EncryptionKey []byte
	// SMTP configures outbound e-mail. It is optional: without SMTP.Host the
	// app does not send mail (see MailEnabled).
	SMTP SMTP
}

// SMTP holds the settings of the outbound mail server. Any provider works
// (Resend, SES, Postmark, Mailpit, a self-hosted server).
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	// TLS is "starttls" (default, port 587), "tls" (implicit, port 465) or
	// "none" (plain text, development only).
	TLS string
	// From is the sender address, e.g. "newsp <no-reply@example.com>".
	From string
}

// SMTP TLS modes accepted in SMTP_TLS.
const (
	SMTPTLSStartTLS = "starttls"
	SMTPTLSImplicit = "tls"
	SMTPTLSNone     = "none"
)

// devEncryptionKey is the documented development default (base64 of 32 bytes).
const devEncryptionKey = "ZGV2LW9ubHktaW5zZWN1cmUtZW5jcnlwdGlvbi1rZXk=" //nolint:gosec // known public value, refused in production

// devJWTSecret is the documented development default; production refuses it.
const devJWTSecret = "dev-only-insecure-jwt-secret-change-me-0123456789" //nolint:gosec // known public value, kept only to refuse it in production

// IsProduction reports whether the app runs with production safeguards.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// MailEnabled reports whether an SMTP server is configured. Without one, mail
// is not sent and the app falls back to logging it.
func (c Config) MailEnabled() bool { return c.SMTP.Host != "" }

// Load reads the configuration from the environment and validates it.
// lookup is os.LookupEnv in production code and a map in tests.
func Load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		return def
	}

	cfg := Config{
		Env:         get("APP_ENV", EnvDevelopment),
		HTTPAddr:    get("HTTP_ADDR", ":8080"),
		DatabaseURL: get("DATABASE_URL", ""),
		LogFormat:   get("LOG_FORMAT", ""),
	}

	var errs []error

	switch cfg.Env {
	case EnvDevelopment, EnvProduction, EnvTest:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be development, production or test, got %q", cfg.Env))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	cfg.DatabaseAllowPlaintext = get("DATABASE_ALLOW_PLAINTEXT", "false") == "true"

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	if cfg.LogFormat == "" {
		cfg.LogFormat = "text"
		if cfg.IsProduction() {
			cfg.LogFormat = "json"
		}
	}
	if cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT must be text or json, got %q", cfg.LogFormat))
	}

	for _, origin := range strings.Split(get("CORS_ALLOWED_ORIGINS", "http://localhost:5173"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, origin)
		}
	}

	timeout, err := time.ParseDuration(get("SHUTDOWN_TIMEOUT", "20s"))
	if err != nil || timeout <= 0 {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration, got %q", get("SHUTDOWN_TIMEOUT", "")))
	}
	cfg.ShutdownTimeout = timeout

	cfg.SwaggerEnabled = get("SWAGGER_ENABLED", fmt.Sprint(!cfg.IsProduction())) == "true"

	cfg.JWTSecret = []byte(get("JWT_SECRET", ""))
	if len(cfg.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET is required and must have at least 32 bytes"))
	}
	cfg.AccessTokenTTL = parseDuration(get("ACCESS_TOKEN_TTL", "15m"), "ACCESS_TOKEN_TTL", &errs)
	cfg.RefreshTokenTTL = parseDuration(get("REFRESH_TOKEN_TTL", "720h"), "REFRESH_TOKEN_TTL", &errs)

	cfg.AppBaseURL = strings.TrimRight(get("APP_BASE_URL", "http://localhost:5173"), "/")
	if u, err := url.Parse(cfg.AppBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, fmt.Errorf("APP_BASE_URL must be an absolute http(s) URL, got %q", cfg.AppBaseURL))
	}

	rawKey := get("ENCRYPTION_KEY", "")
	key, err := base64.StdEncoding.DecodeString(rawKey)
	if err != nil || len(key) != 32 {
		errs = append(errs, errors.New("ENCRYPTION_KEY is required: base64 of 32 random bytes (openssl rand -base64 32)"))
	}
	cfg.EncryptionKey = key

	for _, raw := range strings.Split(get("TRUSTED_PROXIES", "127.0.0.1/32,::1/128"), ",") {
		if raw = strings.TrimSpace(raw); raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("TRUSTED_PROXIES: %w", err))
			continue
		}
		cfg.TrustedProxies = append(cfg.TrustedProxies, prefix)
	}

	cfg.SMTP = SMTP{
		Host:     get("SMTP_HOST", ""),
		Username: get("SMTP_USERNAME", ""),
		Password: get("SMTP_PASSWORD", ""),
		TLS:      strings.ToLower(get("SMTP_TLS", SMTPTLSStartTLS)),
		From:     get("MAIL_FROM", ""),
	}
	port, err := strconv.Atoi(get("SMTP_PORT", "587"))
	if err != nil {
		port = 0
	}
	cfg.SMTP.Port = port
	if cfg.MailEnabled() {
		errs = append(errs, cfg.SMTP.validate()...)
	}

	if cfg.IsProduction() {
		errs = append(errs, cfg.productionErrors()...)
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}
	return cfg, nil
}

// productionErrors refuses development defaults that are unsafe in production.
func (c Config) productionErrors() []error {
	var errs []error
	if u, err := url.Parse(c.DatabaseURL); err == nil && u.Query().Get("sslmode") == "disable" && !c.DatabaseAllowPlaintext {
		errs = append(errs, errors.New("DATABASE_URL must not use sslmode=disable in production (set DATABASE_ALLOW_PLAINTEXT=true only for a database on a private network)"))
	}
	for _, origin := range c.CORSOrigins {
		if origin == "*" {
			errs = append(errs, errors.New("CORS_ALLOWED_ORIGINS must not be * in production"))
		}
	}
	if base64.StdEncoding.EncodeToString(c.EncryptionKey) == devEncryptionKey {
		errs = append(errs, errors.New("ENCRYPTION_KEY must not use the development default in production"))
	}
	if string(c.JWTSecret) == devJWTSecret {
		errs = append(errs, errors.New("JWT_SECRET must not use the development default in production"))
	}
	if u, err := url.Parse(c.AppBaseURL); err == nil && u.Scheme == "http" && !isLoopback(u.Hostname()) {
		errs = append(errs, errors.New("APP_BASE_URL must use https in production (http is accepted only for localhost)"))
	}
	if c.MailEnabled() && c.SMTP.TLS == SMTPTLSNone {
		errs = append(errs, errors.New("SMTP_TLS must not be none in production"))
	}
	return errs
}

// validate checks the SMTP settings; it runs only when SMTP_HOST is set.
func (s SMTP) validate() []error {
	var errs []error
	if s.Port < 1 || s.Port > 65535 {
		errs = append(errs, errors.New("SMTP_PORT must be a number between 1 and 65535"))
	}
	switch s.TLS {
	case SMTPTLSStartTLS, SMTPTLSImplicit, SMTPTLSNone:
	default:
		errs = append(errs, fmt.Errorf("SMTP_TLS must be starttls, tls or none, got %q", s.TLS))
	}
	if s.From == "" {
		errs = append(errs, errors.New("MAIL_FROM is required when SMTP_HOST is set"))
	} else if _, err := nmail.ParseAddress(s.From); err != nil {
		errs = append(errs, fmt.Errorf("MAIL_FROM must be a valid address: %w", err))
	}
	return errs
}

// isLoopback reports whether host names this machine, where plain http is
// acceptable for a local install.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

func parseDuration(raw, key string, errs *[]error) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive duration, got %q", key, raw))
	}
	return d
}

// FromEnv loads the configuration from the process environment.
func FromEnv() (Config, error) { return Load(os.LookupEnv) }
