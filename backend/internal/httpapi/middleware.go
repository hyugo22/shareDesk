package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/hyugo22/sharedesk/backend/internal/auth"
	"github.com/hyugo22/sharedesk/backend/internal/rbac"
)

type ctxKey int

const claimsCtxKey ctxKey = iota

func noStoreCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			http.Error(w, "authentification requise", http.StatusUnauthorized)
			return
		}
		claims, err := s.jwt.Verify(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			http.Error(w, "token invalide ou expiré", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFromContext(r.Context())
			if claims == nil || !rbac.HasPermission(claims.Permissions, perm) {
				http.Error(w, "permission refusée", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func claimsFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(claimsCtxKey).(*auth.Claims)
	return claims
}

// clientIP extrait l'IP source réelle, en tenant compte de X-Forwarded-For
// si le backend est placé derrière un composant de confiance en amont.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
