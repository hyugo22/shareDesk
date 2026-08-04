package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.repos.Roles.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := s.repos.Roles.ListPermissions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, perms)
}

type createRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req createRoleRequest
	if err := readJSON(r, &req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "nom de rôle requis")
		return
	}
	role, err := s.repos.Roles.Create(r.Context(), req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusConflict, "impossible de créer le rôle (nom déjà utilisé ?)")
		return
	}
	claims := claimsFromContext(r.Context())
	s.audit(r, "user", claims.UserID, "roles.create", "role", role.ID, map[string]any{"name": role.Name})
	writeJSON(w, http.StatusCreated, role)
}

func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.repos.Roles.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	claims := claimsFromContext(r.Context())
	s.audit(r, "user", claims.UserID, "roles.delete", "role", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

type setRolePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

func (s *Server) handleSetRolePermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setRolePermissionsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	if err := s.repos.Roles.SetPermissions(r.Context(), id, req.Permissions); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	claims := claimsFromContext(r.Context())
	s.audit(r, "user", claims.UserID, "roles.permissions_change", "role", id, map[string]any{"permissions": req.Permissions})
	w.WriteHeader(http.StatusNoContent)
}
