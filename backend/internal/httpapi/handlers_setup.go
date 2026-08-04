package httpapi

import (
	"net/http"

	"github.com/hyugo22/sharedesk/backend/internal/auth"
	"github.com/hyugo22/sharedesk/backend/internal/models"
)

// handleSetupStatus indique si l'instance n'a encore aucun utilisateur, pour
// que le frontend redirige vers l'assistant de configuration initiale plutôt
// que vers l'écran de connexion.
func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.repos.Users.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": count == 0})
}

type setupRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

// handleSetup crée le tout premier compte administrateur depuis l'interface,
// à la place d'un compte généré automatiquement au démarrage. N'accepte
// aucune requête si l'instance a déjà au moins un utilisateur.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	count, err := s.repos.Users.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "l'instance est déjà configurée")
		return
	}

	var req setupRequest
	if err := readJSON(r, &req); err != nil || req.Email == "" || req.DisplayName == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "champs requis manquants")
		return
	}
	if len(req.Password) < 12 {
		writeError(w, http.StatusBadRequest, "le mot de passe doit faire au moins 12 caractères")
		return
	}

	adminRole, err := s.repos.Roles.GetByName(r.Context(), "admin")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rôle admin introuvable (migrations non appliquées ?)")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	user := &models.User{Email: req.Email, DisplayName: req.DisplayName, PasswordHash: hash, RoleID: adminRole.ID}
	if err := s.repos.Users.Create(r.Context(), user); err != nil {
		writeError(w, http.StatusConflict, "création du compte impossible")
		return
	}

	resp, err := s.issueTokenPair(r, user.ID, user.Email, user.RoleID, "admin")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	s.audit(r, "user", user.ID, "setup.completed", "user", user.ID, map[string]any{"email": user.Email})
	writeJSON(w, http.StatusCreated, resp)
}
