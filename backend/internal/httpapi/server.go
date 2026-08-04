// Package httpapi implémente l'API REST + WebSocket de signalisation exposée
// au frontend et aux agents (enrôlement).
package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/hyugo22/sharedesk/backend/internal/auth"
	"github.com/hyugo22/sharedesk/backend/internal/ca"
	appcrypto "github.com/hyugo22/sharedesk/backend/internal/crypto"
	"github.com/hyugo22/sharedesk/backend/internal/directory"
	"github.com/hyugo22/sharedesk/backend/internal/ratelimit"
	"github.com/hyugo22/sharedesk/backend/internal/repository"
	"github.com/hyugo22/sharedesk/backend/internal/ws"
)

type Server struct {
	repos       *repository.Repositories
	jwt         *auth.JWTIssuer
	sealer      *appcrypto.Sealer
	ca          *ca.CA
	hub         *ws.Hub
	directory   directory.Provider
	loginLimit  *ratelimit.Limiter
	enrollLimit *ratelimit.Limiter
	setupLimit  *ratelimit.Limiter

	accessTTL      time.Duration
	refreshTTL     time.Duration
	allowedOrigins []string
}

func NewServer(repos *repository.Repositories, jwtIssuer *auth.JWTIssuer, sealer *appcrypto.Sealer, caInst *ca.CA, hub *ws.Hub, accessTTL, refreshTTL time.Duration, allowedOrigins []string) *Server {
	return &Server{
		repos:          repos,
		jwt:            jwtIssuer,
		sealer:         sealer,
		ca:             caInst,
		hub:            hub,
		directory:      directory.LocalProvider{},
		loginLimit:     ratelimit.New(10, time.Minute),
		enrollLimit:    ratelimit.New(20, time.Minute),
		setupLimit:     ratelimit.New(10, time.Minute),
		accessTTL:      accessTTL,
		refreshTTL:     refreshTTL,
		allowedOrigins: allowedOrigins,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false, // authentification par Bearer token, pas par cookie
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	r.Route("/api/v1", func(r chi.Router) {
		// Réponses API jamais mises en cache par le navigateur : évite qu'une
		// page affiche des données obsolètes tant qu'un rechargement forcé
		// n'a pas été fait.
		r.Use(noStoreCache)

		r.Get("/setup/status", s.handleSetupStatus)
		r.Post("/setup", s.rateLimited(s.setupLimit, s.handleSetup))

		r.Post("/auth/login", s.rateLimited(s.loginLimit, s.handleLogin))
		r.Post("/auth/refresh", s.handleRefresh)
		r.Post("/auth/logout", s.handleLogout)

		r.Post("/agents/enroll", s.rateLimited(s.enrollLimit, s.handleAgentEnroll))

		// Le navigateur ne peut pas positionner d'en-tête Authorization sur une
		// connexion WebSocket native : le JWT est donc vérifié manuellement à
		// partir du paramètre de requête `token` (voir handleWebSocket).
		r.Get("/ws", s.handleWebSocket)

		r.Group(func(r chi.Router) {
			r.Use(s.RequireAuth)

			r.Get("/me", s.handleMe)
			r.Post("/me/mfa/setup", s.handleMFASetup)
			r.Post("/me/mfa/verify", s.handleMFAVerify)

			r.With(s.RequirePermission("agents.control")).Get("/agents", s.handleListAgents)
			r.With(s.RequirePermission("agents.control")).Get("/agents/{id}", s.handleGetAgent)
			r.With(s.RequirePermission("agents.manage")).Post("/agents/{id}/revoke", s.handleRevokeAgent)
			r.With(s.RequirePermission("agents.manage")).Patch("/agents/{id}/tags", s.handleSetAgentTags)
			r.With(s.RequirePermission("agents.manage")).Post("/agents/enrollment-tokens", s.handleCreateEnrollmentToken)

			r.With(s.RequirePermission("agents.control")).Post("/sessions", s.handleStartSession)
			r.With(s.RequirePermission("agents.control")).Post("/sessions/{id}/end", s.handleEndSession)

			r.With(s.RequirePermission("users.manage")).Get("/users", s.handleListUsers)
			r.With(s.RequirePermission("users.manage")).Post("/users", s.handleCreateUser)
			r.With(s.RequirePermission("users.manage")).Patch("/users/{id}", s.handleUpdateUser)
			r.With(s.RequirePermission("users.manage")).Delete("/users/{id}", s.handleDeleteUser)

			r.With(s.RequirePermission("roles.manage")).Get("/roles", s.handleListRoles)
			r.With(s.RequirePermission("roles.manage")).Post("/roles", s.handleCreateRole)
			r.With(s.RequirePermission("roles.manage")).Delete("/roles/{id}", s.handleDeleteRole)
			r.With(s.RequirePermission("roles.manage")).Get("/roles/{id}/permissions", s.handleGetRolePermissions)
			r.With(s.RequirePermission("roles.manage")).Put("/roles/{id}/permissions", s.handleSetRolePermissions)
			r.With(s.RequirePermission("roles.manage")).Get("/permissions", s.handleListPermissions)

			r.With(s.RequirePermission("audit.read")).Get("/audit-logs", s.handleListAuditLogs)

			r.With(s.RequirePermission("settings.manage")).Get("/settings", s.handleGetSettings)
			r.With(s.RequirePermission("settings.manage")).Put("/settings/{key}", s.handleSetSetting)

			r.With(s.RequirePermission("ldap.manage")).Get("/settings/ldap", s.handleGetLDAPConfig)
			r.With(s.RequirePermission("ldap.manage")).Put("/settings/ldap", s.handleSetLDAPConfig)
			r.With(s.RequirePermission("ldap.manage")).Post("/settings/ldap/test", s.handleTestLDAPConfig)
			r.With(s.RequirePermission("ldap.manage")).Post("/settings/ldap/sync", s.handleSyncLDAP)
		})
	})

	return r
}

func (s *Server) rateLimited(limiter *ratelimit.Limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientIP(r)) {
			http.Error(w, "trop de tentatives, réessayez plus tard", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
