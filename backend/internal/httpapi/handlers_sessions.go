package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type startSessionRequest struct {
	AgentID string `json:"agent_id"`
}

// handleStartSession crée l'enregistrement de session (traçabilité : qui,
// quelle machine, début). La négociation WebRTC se fait ensuite via /ws.
func (s *Server) handleStartSession(w http.ResponseWriter, r *http.Request) {
	var req startSessionRequest
	if err := readJSON(r, &req); err != nil || req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id requis")
		return
	}

	agent, err := s.repos.Agents.GetByID(r.Context(), req.AgentID)
	if err != nil || agent.RevokedAt != nil {
		writeError(w, http.StatusNotFound, "agent introuvable ou révoqué")
		return
	}
	if !s.hub.IsAgentConnected(agent.ID) {
		writeError(w, http.StatusConflict, "agent hors ligne")
		return
	}

	claims := claimsFromContext(r.Context())
	session, err := s.repos.Sessions.Start(r.Context(), agent.ID, claims.UserID, clientIP(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	s.audit(r, "user", claims.UserID, "session.start", "agent", agent.ID, map[string]any{"session_id": session.ID})
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) handleEndSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session, err := s.repos.Sessions.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session introuvable")
		return
	}

	claims := claimsFromContext(r.Context())
	if session.UserID != claims.UserID {
		writeError(w, http.StatusForbidden, "session appartenant à un autre utilisateur")
		return
	}

	if err := s.repos.Sessions.End(r.Context(), id, "ended"); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	s.audit(r, "user", claims.UserID, "session.end", "agent", session.AgentID, map[string]any{"session_id": id})
	w.WriteHeader(http.StatusNoContent)
}
