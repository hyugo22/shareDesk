package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EnrollmentRepo struct{ db *pgxpool.Pool }

func (r *EnrollmentRepo) Create(ctx context.Context, tokenHash, description, createdBy string, expiresAt time.Time) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO enrollment_tokens (token_hash, description, created_by, expires_at)
		VALUES ($1,$2,$3,$4) RETURNING id`, tokenHash, description, createdBy, expiresAt).Scan(&id)
	return id, err
}

// Consume valide un token d'enrôlement à usage unique et le marque comme utilisé
// de façon atomique (empêche une réutilisation concurrente du même token).
// L'agent n'existe pas encore à cet instant (voir LinkAgent).
func (r *EnrollmentRepo) Consume(ctx context.Context, tokenHash string) (id, createdBy string, err error) {
	err = r.db.QueryRow(ctx, `
		UPDATE enrollment_tokens
		SET used_at = now()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING id, coalesce(created_by::text, '')`, tokenHash).Scan(&id, &createdBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", errors.New("token d'enrôlement invalide, expiré ou déjà utilisé")
	}
	return id, createdBy, err
}

// LinkAgent associe a posteriori l'agent créé au token d'enrôlement consommé.
func (r *EnrollmentRepo) LinkAgent(ctx context.Context, tokenID, agentID string) error {
	_, err := r.db.Exec(ctx, `UPDATE enrollment_tokens SET used_by_agent_id = $2 WHERE id = $1`, tokenID, agentID)
	return err
}
