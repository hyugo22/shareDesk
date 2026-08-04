package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hyugo22/sharedesk/backend/internal/auth"
	"github.com/hyugo22/sharedesk/backend/internal/models"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.repos.Users.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type createUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	RoleID      string `json:"role_id"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := readJSON(r, &req); err != nil || req.Email == "" || req.Password == "" || req.RoleID == "" {
		writeError(w, http.StatusBadRequest, "champs requis manquants")
		return
	}
	if len(req.Password) < 12 {
		writeError(w, http.StatusBadRequest, "le mot de passe doit faire au moins 12 caractères")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	user := &models.User{Email: req.Email, DisplayName: req.DisplayName, PasswordHash: hash, RoleID: req.RoleID}
	if err := s.repos.Users.Create(r.Context(), user); err != nil {
		writeError(w, http.StatusConflict, "impossible de créer l'utilisateur (email déjà utilisé ?)")
		return
	}

	claims := claimsFromContext(r.Context())
	s.audit(r, "user", claims.UserID, "users.create", "user", user.ID, map[string]any{"email": user.Email})
	writeJSON(w, http.StatusCreated, user)
}

type updateUserRequest struct {
	RoleID   *string `json:"role_id"`
	IsActive *bool   `json:"is_active"`
	Password *string `json:"password"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateUserRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	claims := claimsFromContext(r.Context())

	if req.RoleID != nil {
		if err := s.repos.Users.UpdateRole(r.Context(), id, *req.RoleID); err != nil {
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		s.audit(r, "user", claims.UserID, "users.role_change", "user", id, map[string]any{"role_id": *req.RoleID})
	}
	if req.IsActive != nil {
		if err := s.repos.Users.SetActive(r.Context(), id, *req.IsActive); err != nil {
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		if !*req.IsActive {
			_ = s.repos.Users.RevokeAllRefreshTokens(r.Context(), id)
		}
		s.audit(r, "user", claims.UserID, "users.active_change", "user", id, map[string]any{"is_active": *req.IsActive})
	}
	if req.Password != nil {
		if len(*req.Password) < 12 {
			writeError(w, http.StatusBadRequest, "le mot de passe doit faire au moins 12 caractères")
			return
		}
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		if err := s.repos.Users.SetPasswordHash(r.Context(), id, hash); err != nil {
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		_ = s.repos.Users.RevokeAllRefreshTokens(r.Context(), id)
		s.audit(r, "user", claims.UserID, "users.password_reset", "user", id, nil)
	}

	w.WriteHeader(http.StatusNoContent)
}
