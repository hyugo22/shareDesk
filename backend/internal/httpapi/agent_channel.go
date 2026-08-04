package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var agentUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true }, // agents ne sont pas des navigateurs
}

// AgentRoutes est monté sur le listener mTLS dédié aux agents (voir cmd/server/main.go).
// L'identité de l'agent est établie par le certificat client vérifié au niveau
// TLS (tls.Config.ClientAuth = RequireAndVerifyClientCert) : aucun jeton
// applicatif n'est nécessaire ni accepté sur ce canal.
func (s *Server) AgentRoutes() http.Handler {
	r := chi.NewRouter()
	r.Get("/agent/channel", s.handleAgentChannel)
	return r
}

func (s *Server) handleAgentChannel(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		writeError(w, http.StatusUnauthorized, "certificat client mTLS requis")
		return
	}
	cert := r.TLS.PeerCertificates[0]
	serial := cert.SerialNumber.Text(16)

	revoked, err := s.repos.Agents.IsCertRevoked(r.Context(), serial)
	if err != nil || revoked {
		writeError(w, http.StatusForbidden, "certificat révoqué")
		return
	}

	agent, err := s.repos.Agents.GetByCertSerial(r.Context(), serial)
	if err != nil || agent.RevokedAt != nil {
		writeError(w, http.StatusForbidden, "agent inconnu ou révoqué")
		return
	}

	conn, err := agentUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.audit(r, "agent", agent.ID, "agent.connected", "agent", agent.ID, nil)
	s.hub.RegisterAgent(agent.ID, conn) // bloquant jusqu'à déconnexion
	s.audit(r, "agent", agent.ID, "agent.disconnected", "agent", agent.ID, nil)
}
