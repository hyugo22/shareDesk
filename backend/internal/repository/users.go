package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyugo22/sharedesk/backend/internal/models"
)

var ErrNotFound = errors.New("ressource introuvable")

type UserRepo struct{ db *pgxpool.Pool }

func (r *UserRepo) Create(ctx context.Context, u *models.User) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO users (email, display_name, password_hash, role_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, is_active, created_at, updated_at`,
		u.Email, u.DisplayName, u.PasswordHash, u.RoleID,
	).Scan(&u.ID, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.scanOne(ctx, `
		SELECT u.id, u.email, u.display_name, u.password_hash, u.role_id, r.name,
		       u.mfa_enabled, coalesce(u.mfa_totp_secret_enc, ''), u.is_active, u.failed_login_attempts,
		       u.locked_until, u.last_login_at, u.created_at, u.updated_at
		FROM users u JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1`, email)
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	return r.scanOne(ctx, `
		SELECT u.id, u.email, u.display_name, u.password_hash, u.role_id, r.name,
		       u.mfa_enabled, coalesce(u.mfa_totp_secret_enc, ''), u.is_active, u.failed_login_attempts,
		       u.locked_until, u.last_login_at, u.created_at, u.updated_at
		FROM users u JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1`, id)
}

func (r *UserRepo) scanOne(ctx context.Context, query string, args ...any) (*models.User, error) {
	var u models.User
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.RoleID, &u.RoleName,
		&u.MFAEnabled, &u.MFATOTPSecretEnc, &u.IsActive, &u.FailedLoginAttempts,
		&u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) List(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.email, u.display_name, u.role_id, r.name, u.mfa_enabled,
		       u.is_active, u.last_login_at, u.created_at, u.updated_at
		FROM users u JOIN roles r ON r.id = u.role_id
		ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.RoleID, &u.RoleName,
			&u.MFAEnabled, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (r *UserRepo) RegisterFailedLogin(ctx context.Context, id string, lockThreshold int, lockDuration time.Duration) error {
	lockUntil := time.Now().Add(lockDuration)
	_, err := r.db.Exec(ctx, `
		UPDATE users SET
			failed_login_attempts = failed_login_attempts + 1,
			locked_until = CASE WHEN failed_login_attempts + 1 >= $2 THEN $3::timestamptz ELSE locked_until END,
			updated_at = now()
		WHERE id = $1`, id, lockThreshold, lockUntil)
	return err
}

func (r *UserRepo) RegisterSuccessfulLogin(ctx context.Context, id, ip string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET
			failed_login_attempts = 0, locked_until = NULL,
			last_login_at = now(), last_login_ip = $2, updated_at = now()
		WHERE id = $1`, id, ip)
	return err
}

func (r *UserRepo) SetMFASecret(ctx context.Context, id, encryptedSecret string, enabled bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET mfa_totp_secret_enc = $2, mfa_enabled = $3, updated_at = now()
		WHERE id = $1`, id, encryptedSecret, enabled)
	return err
}

func (r *UserRepo) UpdateRole(ctx context.Context, id, roleID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET role_id = $2, updated_at = now() WHERE id = $1`, id, roleID)
	return err
}

func (r *UserRepo) SetActive(ctx context.Context, id string, active bool) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	return err
}

func (r *UserRepo) SetPasswordHash(ctx context.Context, id, hash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
	return err
}

// --- Refresh tokens ---

func (r *UserRepo) StoreRefreshToken(ctx context.Context, userID, tokenHash, userAgent, ip string, expiresAt time.Time) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID, tokenHash, expiresAt, userAgent, nullIfEmpty(ip)).Scan(&id)
	return id, err
}

func (r *UserRepo) GetRefreshToken(ctx context.Context, tokenHash string) (userID string, revoked bool, expiresAt time.Time, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT user_id, revoked_at IS NOT NULL, expires_at FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&userID, &revoked, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return
}

func (r *UserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func (r *UserRepo) RevokeAllRefreshTokens(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
