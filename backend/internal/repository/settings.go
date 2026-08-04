package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepo struct{ db *pgxpool.Pool }

func (r *SettingsRepo) Get(ctx context.Context, key string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := r.db.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&raw)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return raw, err
}

func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := r.db.Query(ctx, `SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v json.RawMessage
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (r *SettingsRepo) Set(ctx context.Context, key string, value any, updatedBy string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO app_settings (key, value, updated_by) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		key, raw, nullIfEmpty(updatedBy))
	return err
}
