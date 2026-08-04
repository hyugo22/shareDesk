package httpapi

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var viewerUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// L'authentification repose sur la validité du JWT (voir ci-dessous), pas
	// sur l'origine : le token est à courte durée de vie et transmis en query
	// string faute de pouvoir positionner un en-tête sur une WebSocket native.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleWebSocket relie un navigateur authentifié à l'agent ciblé par une
// session de contrôle déjà créée via POST /sessions.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	sessionID := r.URL.Query().Get("session_id")
	if token == "" || sessionID == "" {
		writeError(w, http.StatusBadRequest, "token et session_id requis")
		return
	}

	claims, err := s.jwt.Verify(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "token invalide ou expiré")
		return
	}

	session, err := s.repos.Sessions.GetByID(r.Context(), sessionID)
	if err != nil || session.UserID != claims.UserID || session.Status != "active" {
		writeError(w, http.StatusForbidden, "session invalide")
		return
	}

	conn, err := viewerUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.hub.RegisterViewer(sessionID, session.AgentID, conn)
}
