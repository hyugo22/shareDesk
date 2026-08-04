package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/hyugo22/sharedesk/backend/internal/auth"
	"github.com/hyugo22/sharedesk/backend/internal/models"
)

type createEnrollmentTokenRequest struct {
	Description string `json:"description"`
	TTLMinutes  int    `json:"ttl_minutes"`
}

func (s *Server) handleCreateEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	var req createEnrollmentTokenRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	if req.TTLMinutes <= 0 || req.TTLMinutes > 24*60 {
		req.TTLMinutes = 60
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	claims := claimsFromContext(r.Context())
	_, err := s.repos.Enrollment.Create(r.Context(), auth.HashToken(token), req.Description, claims.UserID,
		time.Now().Add(time.Duration(req.TTLMinutes)*time.Minute))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	s.audit(r, "user", claims.UserID, "agents.enrollment_token.create", "enrollment_token", "", map[string]any{"description": req.Description})

	// Le token en clair n'est retourné qu'une seule fois, à l'instant de la création.
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"expires_in": req.TTLMinutes * 60,
	})
}

type agentEnrollRequest struct {
	EnrollmentToken string `json:"enrollment_token"`
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	OSVersion       string `json:"os_version"`
	Arch            string `json:"arch"`
	AgentVersion    string `json:"agent_version"`
	CSRPEM          string `json:"csr_pem"`
}

// handleAgentEnroll échange un token d'enrôlement à usage unique contre un
// certificat client mTLS signé par la CA interne. Accessible sans
// authentification préalable (l'agent n'a pas encore d'identité), mais
// protégé par le token à usage unique et un rate limit par IP.
func (s *Server) handleAgentEnroll(w http.ResponseWriter, r *http.Request) {
	var req agentEnrollRequest
	if err := readJSON(r, &req); err != nil || req.EnrollmentToken == "" || req.CSRPEM == "" || req.Hostname == "" {
		writeError(w, http.StatusBadRequest, "champs requis manquants")
		return
	}

	tokenID, _, err := s.repos.Enrollment.Consume(r.Context(), auth.HashToken(req.EnrollmentToken))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	certPEM, serialHex, fingerprint, err := s.ca.SignAgentCSR([]byte(req.CSRPEM), req.Hostname)
	if err != nil {
		writeError(w, http.StatusBadRequest, "CSR invalide: "+err.Error())
		return
	}

	agent := &models.Agent{
		Name: req.Hostname, Hostname: req.Hostname, OS: req.OS, OSVersion: req.OSVersion,
		Arch: req.Arch, AgentVersion: req.AgentVersion,
		CertSerial: serialHex, CertFingerprintSHA256: fingerprint,
	}
	if err := s.repos.Agents.Create(r.Context(), agent, tokenID); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	_ = s.repos.Enrollment.LinkAgent(r.Context(), tokenID, agent.ID)

	s.audit(r, "agent", agent.ID, "agents.enrolled", "agent", agent.ID, map[string]any{"hostname": req.Hostname})

	writeJSON(w, http.StatusCreated, map[string]any{
		"agent_id":    agent.ID,
		"cert_pem":    string(certPEM),
		"ca_cert_pem": string(s.ca.CertPEM()),
	})
}
