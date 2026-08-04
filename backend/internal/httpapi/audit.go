package httpapi

import (
	"log"
	"net/http"

	"github.com/hyugo22/sharedesk/backend/internal/repository"
)

// audit journalise une action dans la table append-only audit_logs.
// actorID est l'ID utilisateur ou agent selon actorType ("system" pour les deux vide).
func (s *Server) audit(r *http.Request, actorType, actorID, action, targetType, targetID string, details map[string]any) {
	entry := repository.AuditEntry{
		ActorType:  actorType,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		IPAddress:  clientIP(r),
		Details:    details,
	}
	switch actorType {
	case "user":
		entry.ActorUserID = actorID
	case "agent":
		entry.ActorAgentID = actorID
	}
	if err := s.repos.Audit.Append(r.Context(), entry); err != nil {
		log.Printf("audit: échec d'écriture du log (action=%s): %v", action, err)
	}
}
