package config

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-with-at-least-32-bytes-000"

// lookupFrom fills JWT_SECRET unless the test sets it, so cases stay focused.
func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		if !ok && key == "JWT_SECRET" {
			return testSecret, true
		}
		if !ok && key == "ENCRYPTION_KEY" {
			return devEncryptionKey, true
		}
		return v, ok
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
		check   func(t *testing.T, cfg Config)
	}{
		{
			name: "development defaults",
			env:  map[string]string{"DATABASE_URL": "postgres://u:p@localhost/db?sslmode=disable"},
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, EnvDevelopment, cfg.Env)
				assert.Equal(t, ":8080", cfg.HTTPAddr)
				assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
				assert.Equal(t, "text", cfg.LogFormat)
				assert.Equal(t, []string{"http://localhost:5173"}, cfg.CORSOrigins)
				assert.Equal(t, 20*time.Second, cfg.ShutdownTimeout)
				assert.True(t, cfg.SwaggerEnabled)
				assert.Equal(t, 15*time.Minute, cfg.AccessTokenTTL)
				assert.Equal(t, 720*time.Hour, cfg.RefreshTokenTTL)
				assert.Equal(t, "http://localhost:5173", cfg.AppBaseURL)
				assert.Len(t, cfg.TrustedProxies, 2)
			},
		},
		{
			name:    "short jwt secret",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": "short"},
			wantErr: "JWT_SECRET",
		},
		{
			name:    "invalid encryption key",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "ENCRYPTION_KEY": "c2hvcnQ="},
			wantErr: "ENCRYPTION_KEY",
		},
		{
			name:    "invalid trusted proxy",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "TRUSTED_PROXIES": "nope"},
			wantErr: "TRUSTED_PROXIES",
		},
		{
			name: "production refuses dev jwt secret",
			env: map[string]string{
				"APP_ENV":      "production",
				"DATABASE_URL": "postgres://u:p@db/newsp?sslmode=require",
				"APP_BASE_URL": "https://newsp.example.com",
				"JWT_SECRET":   devJWTSecret,
			},
			wantErr: "JWT_SECRET must not use the development default",
		},
		{
			name: "production uses json logs and disables swagger",
			env: map[string]string{
				"APP_ENV":              "production",
				"DATABASE_URL":         "postgres://u:p@db/newsp?sslmode=require",
				"CORS_ALLOWED_ORIGINS": "https://newsp.example.com",
				"APP_BASE_URL":         "https://newsp.example.com",
				"ENCRYPTION_KEY":       "cHJvZHVjdGlvbi1rZXktd2l0aC0zMi1ieXRlcyEhISE=",
			},
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, "json", cfg.LogFormat)
				assert.False(t, cfg.SwaggerEnabled)
			},
		},
		{
			name:    "missing database url",
			env:     map[string]string{},
			wantErr: "DATABASE_URL is required",
		},
		{
			name:    "invalid env",
			env:     map[string]string{"APP_ENV": "staging", "DATABASE_URL": "postgres://x"},
			wantErr: "APP_ENV must be",
		},
		{
			name:    "invalid log level",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "LOG_LEVEL": "loud"},
			wantErr: "LOG_LEVEL",
		},
		{
			name: "production refuses sslmode=disable",
			env: map[string]string{
				"APP_ENV":      "production",
				"DATABASE_URL": "postgres://u:p@db/newsp?sslmode=disable",
			},
			wantErr: "sslmode=disable",
		},
		{
			name: "production accepts plaintext database when allowed and http on localhost",
			env: map[string]string{
				"APP_ENV":                  "production",
				"DATABASE_URL":             "postgres://u:p@db/newsp?sslmode=disable",
				"DATABASE_ALLOW_PLAINTEXT": "true",
				"APP_BASE_URL":             "http://localhost:3000",
				"ENCRYPTION_KEY":           "cHJvZHVjdGlvbi1rZXktd2l0aC0zMi1ieXRlcyEhISE=",
			},
			check: func(t *testing.T, cfg Config) {
				assert.True(t, cfg.DatabaseAllowPlaintext)
			},
		},
		{
			name: "production refuses http outside localhost",
			env: map[string]string{
				"APP_ENV":      "production",
				"DATABASE_URL": "postgres://u:p@db/newsp?sslmode=require",
				"APP_BASE_URL": "http://newsp.example.com",
			},
			wantErr: "APP_BASE_URL must use https",
		},
		{
			name: "production refuses wildcard cors",
			env: map[string]string{
				"APP_ENV":              "production",
				"DATABASE_URL":         "postgres://u:p@db/newsp?sslmode=require",
				"CORS_ALLOWED_ORIGINS": "*",
			},
			wantErr: "CORS_ALLOWED_ORIGINS",
		},
		{
			name: "smtp is optional and off by default",
			env:  map[string]string{"DATABASE_URL": "postgres://x"},
			check: func(t *testing.T, cfg Config) {
				assert.False(t, cfg.MailEnabled())
				assert.Equal(t, 587, cfg.SMTP.Port)
				assert.Equal(t, "starttls", cfg.SMTP.TLS)
			},
		},
		{
			name: "smtp values are not validated without a host",
			env:  map[string]string{"DATABASE_URL": "postgres://x", "SMTP_PORT": "nope", "SMTP_TLS": "weird", "MAIL_FROM": "bad"},
			check: func(t *testing.T, cfg Config) {
				assert.False(t, cfg.MailEnabled())
			},
		},
		{
			name: "smtp configured",
			env: map[string]string{
				"DATABASE_URL":  "postgres://x",
				"SMTP_HOST":     "smtp.resend.com",
				"SMTP_PORT":     "465",
				"SMTP_USERNAME": "resend",
				"SMTP_PASSWORD": "re_key",
				"SMTP_TLS":      "TLS",
				"MAIL_FROM":     "newsp <no-reply@example.com>",
			},
			check: func(t *testing.T, cfg Config) {
				assert.True(t, cfg.MailEnabled())
				assert.Equal(t, SMTP{
					Host: "smtp.resend.com", Port: 465, Username: "resend", Password: "re_key",
					TLS: "tls", From: "newsp <no-reply@example.com>",
				}, cfg.SMTP)
			},
		},
		{
			name:    "smtp requires mail from",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "SMTP_HOST": "smtp.example.com"},
			wantErr: "MAIL_FROM is required",
		},
		{
			name:    "smtp rejects invalid mail from",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "SMTP_HOST": "smtp.example.com", "MAIL_FROM": "not-an-address"},
			wantErr: "MAIL_FROM must be a valid address",
		},
		{
			name: "smtp rejects non numeric port",
			env: map[string]string{
				"DATABASE_URL": "postgres://x", "SMTP_HOST": "smtp.example.com",
				"MAIL_FROM": "no-reply@example.com", "SMTP_PORT": "smtp",
			},
			wantErr: "SMTP_PORT",
		},
		{
			name: "smtp rejects port out of range",
			env: map[string]string{
				"DATABASE_URL": "postgres://x", "SMTP_HOST": "smtp.example.com",
				"MAIL_FROM": "no-reply@example.com", "SMTP_PORT": "70000",
			},
			wantErr: "SMTP_PORT",
		},
		{
			name: "smtp rejects port zero",
			env: map[string]string{
				"DATABASE_URL": "postgres://x", "SMTP_HOST": "smtp.example.com",
				"MAIL_FROM": "no-reply@example.com", "SMTP_PORT": "0",
			},
			wantErr: "SMTP_PORT",
		},
		{
			name: "smtp rejects unknown tls mode",
			env: map[string]string{
				"DATABASE_URL": "postgres://x", "SMTP_HOST": "smtp.example.com",
				"MAIL_FROM": "no-reply@example.com", "SMTP_TLS": "ssl",
			},
			wantErr: "SMTP_TLS must be starttls, tls or none",
		},
		{
			name: "smtp accepts tls none in development",
			env: map[string]string{
				"DATABASE_URL": "postgres://x", "SMTP_HOST": "localhost", "SMTP_PORT": "1025",
				"MAIL_FROM": "no-reply@example.com", "SMTP_TLS": "none",
			},
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, "none", cfg.SMTP.TLS)
			},
		},
		{
			name: "production refuses smtp tls none",
			env: map[string]string{
				"APP_ENV":        "production",
				"DATABASE_URL":   "postgres://u:p@db/newsp?sslmode=require",
				"APP_BASE_URL":   "https://newsp.example.com",
				"ENCRYPTION_KEY": "cHJvZHVjdGlvbi1rZXktd2l0aC0zMi1ieXRlcyEhISE=",
				"SMTP_HOST":      "smtp.example.com",
				"MAIL_FROM":      "no-reply@example.com",
				"SMTP_TLS":       "none",
			},
			wantErr: "SMTP_TLS must not be none in production",
		},
		{
			name: "production accepts smtp with starttls and without smtp",
			env: map[string]string{
				"APP_ENV":        "production",
				"DATABASE_URL":   "postgres://u:p@db/newsp?sslmode=require",
				"APP_BASE_URL":   "https://newsp.example.com",
				"ENCRYPTION_KEY": "cHJvZHVjdGlvbi1rZXktd2l0aC0zMi1ieXRlcyEhISE=",
				"SMTP_HOST":      "smtp.example.com",
				"MAIL_FROM":      "no-reply@example.com",
			},
			check: func(t *testing.T, cfg Config) {
				assert.True(t, cfg.MailEnabled())
			},
		},
		{
			name: "production without smtp does not require tls setting",
			env: map[string]string{
				"APP_ENV":        "production",
				"DATABASE_URL":   "postgres://u:p@db/newsp?sslmode=require",
				"APP_BASE_URL":   "https://newsp.example.com",
				"ENCRYPTION_KEY": "cHJvZHVjdGlvbi1rZXktd2l0aC0zMi1ieXRlcyEhISE=",
				"SMTP_TLS":       "none",
			},
			check: func(t *testing.T, cfg Config) {
				assert.False(t, cfg.MailEnabled())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(lookupFrom(tt.env))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}
