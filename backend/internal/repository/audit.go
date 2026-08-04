package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyugo22/sharedesk/backend/internal/models"
)

type AuditRepo struct{ db *pgxpool.Pool }

type AuditEntry struct {
	ActorType    string // "user" | "agent" | "system"
	ActorUserID  string
	ActorAgentID string
	Action       string
	TargetType   string
	TargetID     string
	IPAddress    string
	Details      map[string]any
}

// Append écrit une entrée d'audit. La table audit_logs est append-only au
// niveau base (triggers rejetant UPDATE/DELETE) : aucune méthode de
// modification/suppression n'est exposée ici volontairement.
func (r *AuditRepo) Append(ctx context.Context, e AuditEntry) error {
	details := e.Details
	if details == nil {
		details = map[string]any{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO audit_logs (actor_type, actor_user_id, actor_agent_id, action, target_type, target_id, ip_address, details)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ActorType, nullIfEmpty(e.ActorUserID), nullIfEmpty(e.ActorAgentID), e.Action,
		nullIfEmpty(e.TargetType), nullIfEmpty(e.TargetID), nullIfEmpty(e.IPAddress), raw)
	return err
}

type AuditFilter struct {
	ActorUserID string
	Action      string
	From, To    *time.Time
	Limit       int
	Offset      int
}

func (r *AuditRepo) List(ctx context.Context, f AuditFilter) ([]models.AuditLog, error) {
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 200
	}
	// Résout le nom lisible de l'acteur (utilisateur ou agent) plutôt que de
	// n'exposer que son UUID en base, pour que les logs d'audit restent
	// exploitables sans avoir à recouper manuellement avec la base.
	rows, err := r.db.Query(ctx, `
		SELECT al.id, al.occurred_at, al.actor_type, al.actor_user_id, al.actor_agent_id, al.action,
		       coalesce(al.target_type,''), coalesce(al.target_id,''), host(al.ip_address), al.details,
		       coalesce(nullif(u.display_name, ''), nullif(u.email, ''), nullif(ag.name, ''))
		FROM audit_logs al
		LEFT JOIN users u ON u.id = al.actor_user_id
		LEFT JOIN agents ag ON ag.id = al.actor_agent_id
		WHERE ($1 = '' OR al.actor_user_id::text = $1)
		  AND ($2 = '' OR al.action = $2)
		  AND ($3::timestamptz IS NULL OR al.occurred_at >= $3)
		  AND ($4::timestamptz IS NULL OR al.occurred_at <= $4)
		ORDER BY al.occurred_at DESC
		LIMIT $5 OFFSET $6`,
		f.ActorUserID, f.Action, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		var actorUserID, actorAgentID, ip, actorName *string
		var detailsRaw []byte
		if err := rows.Scan(&l.ID, &l.OccurredAt, &l.ActorType, &actorUserID, &actorAgentID,
			&l.Action, &l.TargetType, &l.TargetID, &ip, &detailsRaw, &actorName); err != nil {
			return nil, err
		}
		l.ActorUserID, l.ActorAgentID = actorUserID, actorAgentID
		if ip != nil {
			l.IPAddress = *ip
		}
		if actorName != nil {
			l.ActorName = *actorName
		} else if l.ActorType == "system" {
			l.ActorName = "Système"
		}
		if len(detailsRaw) > 0 {
			_ = json.Unmarshal(detailsRaw, &l.Details)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
