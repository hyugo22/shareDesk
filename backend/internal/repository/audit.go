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
	rows, err := r.db.Query(ctx, `
		SELECT id, occurred_at, actor_type, actor_user_id, actor_agent_id, action,
		       coalesce(target_type,''), coalesce(target_id,''), host(ip_address), details
		FROM audit_logs
		WHERE ($1 = '' OR actor_user_id::text = $1)
		  AND ($2 = '' OR action = $2)
		  AND ($3::timestamptz IS NULL OR occurred_at >= $3)
		  AND ($4::timestamptz IS NULL OR occurred_at <= $4)
		ORDER BY occurred_at DESC
		LIMIT $5 OFFSET $6`,
		f.ActorUserID, f.Action, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		var actorUserID, actorAgentID, ip *string
		var detailsRaw []byte
		if err := rows.Scan(&l.ID, &l.OccurredAt, &l.ActorType, &actorUserID, &actorAgentID,
			&l.Action, &l.TargetType, &l.TargetID, &ip, &detailsRaw); err != nil {
			return nil, err
		}
		l.ActorUserID, l.ActorAgentID = actorUserID, actorAgentID
		if ip != nil {
			l.IPAddress = *ip
		}
		if len(detailsRaw) > 0 {
			_ = json.Unmarshal(detailsRaw, &l.Details)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
