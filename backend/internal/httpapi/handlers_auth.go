package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hyugo22/sharedesk/backend/internal/auth"
	"github.com/hyugo22/sharedesk/backend/internal/repository"
)

const maxFailedLoginAttempts = 5
const lockDuration = 15 * time.Minute

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}

	user, err := s.repos.Users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		s.audit(r, "system", "", "auth.login.failure", "user", req.Email, map[string]any{"reason": "unknown_email"})
		writeError(w, http.StatusUnauthorized, "identifiants invalides")
		return
	}
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		s.audit(r, "user", user.ID, "auth.login.failure", "user", user.ID, map[string]any{"reason": "account_locked"})
		writeError(w, http.StatusLocked, "compte temporairement verrouillé")
		return
	}
	if !user.IsActive {
		s.audit(r, "user", user.ID, "auth.login.failure", "user", user.ID, map[string]any{"reason": "inactive"})
		writeError(w, http.StatusUnauthorized, "identifiants invalides")
		return
	}

	if err := auth.VerifyPassword(req.Password, user.PasswordHash); err != nil {
		_ = s.repos.Users.RegisterFailedLogin(r.Context(), user.ID, maxFailedLoginAttempts, lockDuration)
		s.audit(r, "user", user.ID, "auth.login.failure", "user", user.ID, map[string]any{"reason": "bad_password"})
		writeError(w, http.StatusUnauthorized, "identifiants invalides")
		return
	}

	if user.MFAEnabled {
		if req.TOTPCode == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "mfa_required"})
			return
		}
		secret, err := s.sealer.OpenString(user.MFATOTPSecretEnc)
		if err != nil || !auth.VerifyTOTPCode(secret, req.TOTPCode) {
			_ = s.repos.Users.RegisterFailedLogin(r.Context(), user.ID, maxFailedLoginAttempts, lockDuration)
			s.audit(r, "user", user.ID, "auth.login.failure", "user", user.ID, map[string]any{"reason": "bad_totp"})
			writeError(w, http.StatusUnauthorized, "code MFA invalide")
			return
		}
	}

	if err := s.repos.Users.RegisterSuccessfulLogin(r.Context(), user.ID, ip); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	resp, err := s.issueTokenPair(r, user.ID, user.Email, user.RoleID, user.RoleName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	s.audit(r, "user", user.ID, "auth.login.success", "user", user.ID, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) issueTokenPair(r *http.Request, userID, email, roleID, roleName string) (*tokenResponse, error) {
	perms, err := s.repos.Roles.PermissionsForRole(r.Context(), roleID)
	if err != nil {
		return nil, err
	}
	access, err := s.jwt.Issue(userID, email, roleName, perms)
	if err != nil {
		return nil, err
	}
	refresh, err := auth.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	if _, err := s.repos.Users.StoreRefreshToken(r.Context(), userID, auth.HashToken(refresh),
		r.UserAgent(), clientIP(r), time.Now().Add(s.refreshTTL)); err != nil {
		return nil, err
	}
	return &tokenResponse{AccessToken: access, RefreshToken: refresh, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := readJSON(r, &req); err != nil || req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}

	hash := auth.HashToken(req.RefreshToken)
	userID, revoked, expiresAt, err := s.repos.Users.GetRefreshToken(r.Context(), hash)
	if err != nil || revoked || expiresAt.Before(time.Now()) {
		writeError(w, http.StatusUnauthorized, "refresh token invalide")
		return
	}

	user, err := s.repos.Users.GetByID(r.Context(), userID)
	if err != nil || !user.IsActive {
		writeError(w, http.StatusUnauthorized, "compte inactif")
		return
	}

	// Rotation : l'ancien refresh token est révoqué dès qu'il est utilisé.
	_ = s.repos.Users.RevokeRefreshToken(r.Context(), hash)

	resp, err := s.issueTokenPair(r, user.ID, user.Email, user.RoleID, user.RoleName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := readJSON(r, &req); err == nil && req.RefreshToken != "" {
		_ = s.repos.Users.RevokeRefreshToken(r.Context(), auth.HashToken(req.RefreshToken))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	user, err := s.repos.Users.GetByID(r.Context(), claims.UserID)
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "utilisateur introuvable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": user.ID, "email": user.Email, "display_name": user.DisplayName,
		"role": user.RoleName, "mfa_enabled": user.MFAEnabled, "permissions": claims.Permissions,
	})
}

func (s *Server) handleMFASetup(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	secret, url, err := auth.GenerateTOTPSecret("ShareDesk", claims.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	enc, err := s.sealer.SealString(secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	if err := s.repos.Users.SetMFASecret(r.Context(), claims.UserID, enc, false); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"otpauth_url": url})
}

type mfaVerifyRequest struct {
	Code string `json:"code"`
}

func (s *Server) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	var req mfaVerifyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	user, err := s.repos.Users.GetByID(r.Context(), claims.UserID)
	if err != nil || user.MFATOTPSecretEnc == "" {
		writeError(w, http.StatusBadRequest, "aucun enrôlement MFA en cours")
		return
	}
	secret, err := s.sealer.OpenString(user.MFATOTPSecretEnc)
	if err != nil || !auth.VerifyTOTPCode(secret, req.Code) {
		writeError(w, http.StatusUnauthorized, "code MFA invalide")
		return
	}
	if err := s.repos.Users.SetMFASecret(r.Context(), claims.UserID, user.MFATOTPSecretEnc, true); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	s.audit(r, "user", claims.UserID, "auth.mfa.enabled", "user", claims.UserID, nil)
	w.WriteHeader(http.StatusNoContent)
}
