package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.repos.Settings.GetAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var value json.RawMessage
	if err := readJSON(r, &value); err != nil {
		writeError(w, http.StatusBadRequest, "valeur JSON invalide")
		return
	}

	claims := claimsFromContext(r.Context())
	if err := s.repos.Settings.Set(r.Context(), key, value, claims.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	s.audit(r, "user", claims.UserID, "settings.update", "setting", key, map[string]any{"value": value})
	w.WriteHeader(http.StatusNoContent)
}
