package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyugo22/sharedesk/backend/internal/models"
)

type AgentRepo struct{ db *pgxpool.Pool }

func (r *AgentRepo) Create(ctx context.Context, a *models.Agent, enrollmentTokenID string) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO agents (name, hostname, os, os_version, arch, agent_version, cert_serial,
		                     cert_fingerprint_sha256, enrollment_token_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, status, enrolled_at, created_at, updated_at`,
		a.Name, a.Hostname, a.OS, a.OSVersion, a.Arch, a.AgentVersion, a.CertSerial,
		a.CertFingerprintSHA256, enrollmentTokenID,
	).Scan(&a.ID, &a.Status, &a.EnrolledAt, &a.CreatedAt, &a.UpdatedAt)
}

func (r *AgentRepo) List(ctx context.Context) ([]models.Agent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, hostname, os, os_version, arch, agent_version, tags, status,
		       cert_serial, cert_fingerprint_sha256, enrolled_at, last_seen_at, revoked_at,
		       coalesce(revoked_reason,''), created_at, updated_at
		FROM agents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := []models.Agent{}
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Hostname, &a.OS, &a.OSVersion, &a.Arch, &a.AgentVersion,
			&a.Tags, &a.Status, &a.CertSerial, &a.CertFingerprintSHA256, &a.EnrolledAt, &a.LastSeenAt,
			&a.RevokedAt, &a.RevokedReason, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (r *AgentRepo) GetByID(ctx context.Context, id string) (*models.Agent, error) {
	var a models.Agent
	err := r.db.QueryRow(ctx, `
		SELECT id, name, hostname, os, os_version, arch, agent_version, tags, status,
		       cert_serial, cert_fingerprint_sha256, enrolled_at, last_seen_at, revoked_at,
		       coalesce(revoked_reason,''), created_at, updated_at
		FROM agents WHERE id = $1`, id).Scan(
		&a.ID, &a.Name, &a.Hostname, &a.OS, &a.OSVersion, &a.Arch, &a.AgentVersion,
		&a.Tags, &a.Status, &a.CertSerial, &a.CertFingerprintSHA256, &a.EnrolledAt, &a.LastSeenAt,
		&a.RevokedAt, &a.RevokedReason, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

// GetByCertSerial identifie un agent lors d'une connexion mTLS à partir du certificat présenté.
func (r *AgentRepo) GetByCertSerial(ctx context.Context, serial string) (*models.Agent, error) {
	var a models.Agent
	err := r.db.QueryRow(ctx, `
		SELECT id, name, hostname, os, os_version, arch, agent_version, tags, status,
		       cert_serial, cert_fingerprint_sha256, enrolled_at, last_seen_at, revoked_at,
		       coalesce(revoked_reason,''), created_at, updated_at
		FROM agents WHERE cert_serial = $1`, serial).Scan(
		&a.ID, &a.Name, &a.Hostname, &a.OS, &a.OSVersion, &a.Arch, &a.AgentVersion,
		&a.Tags, &a.Status, &a.CertSerial, &a.CertFingerprintSHA256, &a.EnrolledAt, &a.LastSeenAt,
		&a.RevokedAt, &a.RevokedReason, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

func (r *AgentRepo) SetStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE agents SET status = $2, last_seen_at = now(), updated_at = now() WHERE id = $1`, id, status)
	return err
}

func (r *AgentRepo) SetTags(ctx context.Context, id string, tags []string) error {
	_, err := r.db.Exec(ctx, `UPDATE agents SET tags = $2, updated_at = now() WHERE id = $1`, id, tags)
	return err
}

// Revoke marque l'agent comme révoqué et l'ajoute à la CRL applicative,
// vérifiée à chaque tentative de connexion mTLS (voir internal/ca).
func (r *AgentRepo) Revoke(ctx context.Context, id, reason string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var certSerial string
	if err := tx.QueryRow(ctx, `
		UPDATE agents SET revoked_at = now(), revoked_reason = $2, status = 'offline', updated_at = now()
		WHERE id = $1 RETURNING cert_serial`, id, reason).Scan(&certSerial); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO agent_cert_revocations (cert_serial, agent_id, reason) VALUES ($1,$2,$3)
		ON CONFLICT (cert_serial) DO NOTHING`, certSerial, id, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *AgentRepo) IsCertRevoked(ctx context.Context, certSerial string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_cert_revocations WHERE cert_serial = $1)`, certSerial).Scan(&exists)
	return exists, err
}

func (r *AgentRepo) MarkStaleOffline(ctx context.Context, staleAfter time.Duration) error {
	_, err := r.db.Exec(ctx, `
		UPDATE agents SET status = 'offline', updated_at = now()
		WHERE status = 'online' AND last_seen_at < $1`, time.Now().Add(-staleAfter))
	return err
}
