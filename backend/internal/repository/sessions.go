package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyugo22/sharedesk/backend/internal/models"
)

type SessionRepo struct{ db *pgxpool.Pool }

func (r *SessionRepo) Start(ctx context.Context, agentID, userID, ip string) (*models.ControlSession, error) {
	var s models.ControlSession
	s.AgentID, s.UserID = agentID, userID
	err := r.db.QueryRow(ctx, `
		INSERT INTO control_sessions (agent_id, user_id, ip_address) VALUES ($1,$2,$3)
		RETURNING id, status, started_at`, agentID, userID, nullIfEmpty(ip)).
		Scan(&s.ID, &s.Status, &s.StartedAt)
	return &s, err
}

func (r *SessionRepo) GetByID(ctx context.Context, id string) (*models.ControlSession, error) {
	var s models.ControlSession
	err := r.db.QueryRow(ctx, `
		SELECT id, agent_id, user_id, status, started_at, ended_at
		FROM control_sessions WHERE id = $1`, id).
		Scan(&s.ID, &s.AgentID, &s.UserID, &s.Status, &s.StartedAt, &s.EndedAt)
	if err != nil {
		return nil, ErrNotFound
	}
	return &s, nil
}

func (r *SessionRepo) End(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE control_sessions SET status = $2, ended_at = now() WHERE id = $1 AND ended_at IS NULL`, id, status)
	return err
}

func (r *SessionRepo) ListForAgent(ctx context.Context, agentID string) ([]models.ControlSession, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, agent_id, user_id, status, started_at, ended_at
		FROM control_sessions WHERE agent_id = $1 ORDER BY started_at DESC LIMIT 100`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []models.ControlSession{}
	for rows.Next() {
		var s models.ControlSession
		if err := rows.Scan(&s.ID, &s.AgentID, &s.UserID, &s.Status, &s.StartedAt, &s.EndedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
