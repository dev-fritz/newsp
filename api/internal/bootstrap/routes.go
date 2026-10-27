package bootstrap

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/dev-fritz/newsp/api/docs" // registers the generated Swagger spec
	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/middlewares"
)

// NewRouter registers every route and middleware.
func NewRouter(c *Container) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middlewares.Recoverer(c.Logger))
	r.Use(middlewares.ClientIP(c.Config.TrustedProxies))
	r.Use(middlewares.RequestLogger(c.Logger))
	r.Use(middlewares.SecurityHeaders(c.Config.IsProduction()))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   c.Config.CORSOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", c.HealthHandler.Live)
	r.Get("/readyz", c.HealthHandler.Ready)

	if c.Config.SwaggerEnabled {
		r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	}

	r.Route("/api/v1", func(r chi.Router) {
		// Refresh and logout prove possession of a refresh token; the web BFF
		// calls them for every user, so they are not limited per IP.
		r.Post("/auth/refresh", c.AuthHandler.Refresh)
		r.Post("/auth/logout", c.AuthHandler.Logout)

		// Unauthenticated endpoints, rate limited per client IP.
		r.Group(func(r chi.Router) {
			r.Use(middlewares.RateLimitByIP(c.AuthLimiter, "auth"))
			r.Get("/setup/status", c.AuthHandler.SetupStatus)
			r.Post("/setup", c.AuthHandler.Setup)
			r.Post("/auth/login", c.AuthHandler.Login)
			r.With(middlewares.RateLimitByIP(c.SignupLimiter, "signup")).Post("/auth/signup", c.AuthHandler.Signup)
			r.Post("/auth/verify-email", c.AuthHandler.VerifyEmail)
			r.Post("/invites/inspect", c.AuthHandler.InspectInvite)
			r.Post("/invites/accept", c.AuthHandler.AcceptInvite)
			r.Post("/password-resets/inspect", c.AuthHandler.InspectPasswordReset)
			r.With(middlewares.RateLimitByIP(c.ResetLimiter, "password-reset")).Post("/password-resets/request", c.AuthHandler.RequestPasswordReset)
			r.Post("/password-resets/confirm", c.AuthHandler.ResetPassword)
		})

		r.Group(func(r chi.Router) {
			r.Use(middlewares.RequireAuth(c.TokenIssuer))

			r.Get("/me", c.AuthHandler.Me)
			r.Put("/me/password", c.AuthHandler.ChangePassword)
			r.Post("/me/verify-email/resend", c.AuthHandler.ResendVerification)
			r.Get("/jobs/{id}", c.JobHandler.Get)
			r.Get("/ai/models", c.AIHandler.ChatModels)
			r.Get("/me/preferences", c.PreferenceHandler.Get)
			r.Put("/me/preferences", c.PreferenceHandler.Update)

			r.Get("/reports", c.ReportHandler.List)
			r.Post("/reports", c.ReportHandler.Create)
			r.Get("/reports/{id}", c.ReportHandler.Get)
			r.Delete("/reports/{id}", c.ReportHandler.Delete)

			r.Get("/ideas", c.IdeaHandler.List)
			r.Post("/ideas/generate", c.IdeaHandler.Generate)
			r.Get("/ideas/{id}", c.IdeaHandler.Get)
			r.Put("/ideas/{id}", c.IdeaHandler.Edit)
			r.Patch("/ideas/{id}/status", c.IdeaHandler.SetStatus)
			r.Post("/ideas/{id}/deepen", c.IdeaHandler.Deepen)
			r.Get("/ideas/{id}/versions/{version}", c.IdeaHandler.GetVersion)
			r.Get("/sources", c.SourceHandler.ListPublic)
			r.Get("/items", c.ItemHandler.List)

			r.Route("/admin", func(r chi.Router) {
				r.With(middlewares.RequirePermission(auth.PermUsersManage)).Group(func(r chi.Router) {
					r.Get("/users", c.UserHandler.List)
					r.Patch("/users/{id}", c.UserHandler.UpdateAccess)
					r.Post("/users/{id}/password-reset", c.UserHandler.CreatePasswordReset)
					r.Get("/invites", c.UserHandler.ListInvites)
					r.Post("/invites", c.UserHandler.CreateInvite)
					r.Delete("/invites/{id}", c.UserHandler.RevokeInvite)
				})
				r.With(middlewares.RequirePermission(auth.PermSourcesManage)).Route("/sources", func(r chi.Router) {
					r.Get("/", c.SourceHandler.List)
					r.Post("/", c.SourceHandler.Create)
					r.Patch("/{id}", c.SourceHandler.Update)
					r.Delete("/{id}", c.SourceHandler.Delete)
					r.Post("/{id}/collect", c.SourceHandler.Collect)
				})
				r.With(middlewares.RequirePermission(auth.PermProvidersManage)).Route("/ai", func(r chi.Router) {
					r.Get("/providers", c.AIHandler.ListProviders)
					r.Post("/providers", c.AIHandler.CreateProvider)
					r.Patch("/providers/{id}", c.AIHandler.UpdateProvider)
					r.Delete("/providers/{id}", c.AIHandler.DeleteProvider)
					r.Post("/providers/{id}/test", c.AIHandler.TestConnection)
					r.Post("/providers/{id}/models", c.AIHandler.CreateModel)
					r.Patch("/models/{id}", c.AIHandler.UpdateModel)
					r.Delete("/models/{id}", c.AIHandler.DeleteModel)
				})
				r.With(middlewares.RequirePermission(auth.PermSettingsManage)).Group(func(r chi.Router) {
					r.Get("/settings", c.AIHandler.GetSettings)
					r.Patch("/settings", c.AIHandler.UpdateSettings)
				})
			})
		})
	})

	return r
}
