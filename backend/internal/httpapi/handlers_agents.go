package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.repos.Agents.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	agent, err := s.repos.Agents.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "agent introuvable")
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

type revokeAgentRequest struct {
	Reason string `json:"reason"`
}

// handleRevokeAgent révoque immédiatement le certificat mTLS d'un agent :
// toute connexion ultérieure de cet agent sera rejetée (CRL applicative
// vérifiée par le listener mTLS, voir internal/ws) et sa session active est coupée.
func (s *Server) handleRevokeAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req revokeAgentRequest
	_ = readJSON(r, &req)

	if err := s.repos.Agents.Revoke(r.Context(), id, req.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	s.hub.DisconnectAgent(id)

	claims := claimsFromContext(r.Context())
	s.audit(r, "user", claims.UserID, "agents.revoke", "agent", id, map[string]any{"reason": req.Reason})
	w.WriteHeader(http.StatusNoContent)
}

type setAgentTagsRequest struct {
	Tags []string `json:"tags"`
}

func (s *Server) handleSetAgentTags(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setAgentTagsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	if err := s.repos.Agents.SetTags(r.Context(), id, req.Tags); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
