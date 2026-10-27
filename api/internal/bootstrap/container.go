// Package bootstrap wires dependencies (manual DI) and the HTTP routes.
package bootstrap

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/config"
	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/handlers"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/pkg/ai"
	"github.com/dev-fritz/newsp/api/pkg/connectors"
	"github.com/dev-fritz/newsp/api/pkg/extract"
	"github.com/dev-fritz/newsp/api/pkg/mail"
	"github.com/dev-fritz/newsp/api/pkg/ratelimit"
	"github.com/dev-fritz/newsp/api/pkg/safehttp"
	"github.com/dev-fritz/newsp/api/pkg/secretbox"
)

// Container holds the application's long-lived dependencies.
type Container struct {
	Config config.Config
	Logger *slog.Logger
	DB     *sql.DB
	Tx     *database.TxManager
	Now    func() time.Time

	TokenIssuer *auth.TokenIssuer
	// AuthLimiter bounds unauthenticated auth requests per client IP.
	AuthLimiter *ratelimit.Limiter
	// SignupLimiter bounds account creation per client IP.
	SignupLimiter *ratelimit.Limiter
	// ResetLimiter bounds password reset requests per client IP.
	ResetLimiter *ratelimit.Limiter

	Jobs  *repositories.JobRepository
	Queue *workers.Queue
	// AI is how features call models; tests replace its provider factory.
	AI *services.AIGateway
	// Sources runs source administration and the collection and embedding jobs.
	Sources *services.SourceService
	// Auth runs sign-in and the password reset job.
	Auth *services.AuthService
	// Reports and Ideas run the generation jobs.
	Reports *services.ReportService
	Ideas   *services.IdeaService
	// Maintenance runs data retention and embedding reindexing.
	Maintenance *services.MaintenanceService
	// Emails queues transactional e-mails and delivers them from the worker.
	Emails *services.EmailService

	HealthHandler     *handlers.HealthHandler
	AuthHandler       *handlers.AuthHandler
	UserHandler       *handlers.UserHandler
	JobHandler        *handlers.JobHandler
	AIHandler         *handlers.AIHandler
	SourceHandler     *handlers.SourceHandler
	ItemHandler       *handlers.ItemHandler
	PreferenceHandler *handlers.PreferenceHandler
	ReportHandler     *handlers.ReportHandler
	IdeaHandler       *handlers.IdeaHandler
}

// Options replace external dependencies, mostly in tests. Zero values select
// the production ones.
type Options struct {
	// AIFactory builds AI provider clients; nil uses ai.New.
	AIFactory services.ProviderFactory
	// HTTPClient fetches every source; nil uses an SSRF-safe client.
	HTTPClient *http.Client
	// Mailer sends e-mail; nil uses SMTP when configured, else the log mailer.
	Mailer services.Mailer
}

// NewContainer builds every service and handler by constructor injection.
// opts swaps external dependencies; the zero value gives the real ones.
func NewContainer(cfg config.Config, logger *slog.Logger, db *sql.DB, opts Options) (*Container, error) {
	factory := opts.AIFactory
	if factory == nil {
		factory = ai.New
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = safehttp.NewClient(safehttp.Options{})
	}
	box, err := secretbox.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}

	now := func() time.Time { return time.Now().UTC() }
	tx := database.NewTxManager(db)

	users := repositories.NewUserRepository(db)
	userTokens := repositories.NewUserTokenRepository(db)
	sessions := repositories.NewRefreshTokenRepository(db)
	settings := repositories.NewSettingsRepository(db)
	jobs := repositories.NewJobRepository(db)
	queue := workers.NewQueue(jobs, now)
	// Without a mail server, development still shows the links in the log.
	emails := services.NewEmailService(queue, box, newMailer(cfg, logger, opts.Mailer), jobs, cfg.MailEnabled() || !cfg.IsProduction())
	aiStore := repositories.NewAIRepository(db)
	gateway := services.NewAIGateway(aiStore, settings, box, factory, now)
	aiAdmin := services.NewAIAdminService(tx, aiStore, settings, box, factory, queue, emails.Ready(), now)

	sourceRepo := repositories.NewSourceRepository(db)
	itemRepo := repositories.NewItemRepository(db)
	registry := connectors.NewRegistry(
		connectors.NewHackerNews(httpClient, ""),
		connectors.NewRSS(httpClient, now),
		connectors.NewTabNews(httpClient, ""),
		connectors.NewReddit(httpClient, "", now),
		connectors.NewYouTube(httpClient, "", now),
		connectors.NewLobsters(httpClient, ""),
		connectors.NewDevTo(httpClient, ""),
		connectors.NewMastodon(httpClient, ""),
		connectors.NewBluesky(httpClient, "", "", now),
		connectors.NewGitHub(httpClient, "", now),
		connectors.NewProductHunt(httpClient, "", now),
	)
	sources := services.NewSourceService(sourceRepo, itemRepo, registry, gateway, queue, box, now)

	prefRepo := repositories.NewPreferenceRepository(db)
	candidateRepo := repositories.NewCandidateRepository(db)
	extractor := extract.New(httpClient)
	preferences := services.NewPreferenceService(tx, prefRepo, users, aiStore, settings, gateway, now)
	reportRepo := repositories.NewReportRepository(db)
	reports := services.NewReportService(tx, reportRepo, candidateRepo, prefRepo, prefRepo,
		preferences, gateway, extractor, queue, now)
	ideas := services.NewIdeaService(tx, repositories.NewIdeaRepository(db), candidateRepo, reportRepo, prefRepo, prefRepo,
		preferences, gateway, extractor, settings, queue, now)

	maintenance := services.NewMaintenanceService(repositories.NewMaintenanceRepository(db), settings, gateway, queue, now)

	issuer := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL, now)
	loginLimiter := ratelimit.New(10, 15*time.Minute, now) // per e-mail

	resendLimiter := ratelimit.New(3, time.Hour, now) // per user
	resetLimiter := ratelimit.New(3, time.Hour, now)  // per e-mail
	authService := services.NewAuthService(tx, users, userTokens, sessions, settings, prefRepo, issuer, loginLimiter, resendLimiter, resetLimiter,
		emails, queue, cfg.AppBaseURL, cfg.RefreshTokenTTL, now)
	userService := services.NewUserService(tx, users, userTokens, sessions, emails, cfg.AppBaseURL, now)

	return &Container{
		AI:                gateway,
		AIHandler:         handlers.NewAIHandler(aiAdmin),
		Sources:           sources,
		SourceHandler:     handlers.NewSourceHandler(sources),
		ItemHandler:       handlers.NewItemHandler(services.NewItemService(itemRepo)),
		Reports:           reports,
		Ideas:             ideas,
		Maintenance:       maintenance,
		Emails:            emails,
		PreferenceHandler: handlers.NewPreferenceHandler(preferences),
		ReportHandler:     handlers.NewReportHandler(reports),
		IdeaHandler:       handlers.NewIdeaHandler(ideas),
		Config:            cfg,
		Logger:            logger,
		DB:                db,
		Tx:                tx,
		Now:               now,
		TokenIssuer:       issuer,
		AuthLimiter:       ratelimit.New(60, 10*time.Minute, now), // per IP
		SignupLimiter:     ratelimit.New(5, time.Hour, now),       // per IP
		ResetLimiter:      ratelimit.New(5, time.Hour, now),       // per IP
		Jobs:              jobs,
		Queue:             queue,
		HealthHandler:     handlers.NewHealthHandler(db),
		Auth:              authService,
		AuthHandler:       handlers.NewAuthHandler(authService),
		UserHandler:       handlers.NewUserHandler(userService),
		JobHandler:        handlers.NewJobHandler(services.NewJobService(jobs)),
	}, nil
}

// newMailer picks the e-mail sender: override (tests), SMTP when configured,
// else a log mailer that prints the body (and its links) only in development.
func newMailer(cfg config.Config, logger *slog.Logger, override services.Mailer) services.Mailer {
	if override != nil {
		return override
	}
	if !cfg.MailEnabled() {
		return mail.NewLog(logger, !cfg.IsProduction())
	}
	return mail.NewSMTP(mail.SMTPConfig{
		Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username, Password: cfg.SMTP.Password,
		TLS: mail.TLSMode(cfg.SMTP.TLS), From: cfg.SMTP.From,
	})
}

// NewLogger builds the process logger: JSON in production, text in development.
func NewLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var h slog.Handler = slog.NewTextHandler(os.Stdout, opts)
	if cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}
