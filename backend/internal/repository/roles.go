package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyugo22/sharedesk/backend/internal/models"
)

type RoleRepo struct{ db *pgxpool.Pool }

func (r *RoleRepo) List(ctx context.Context) ([]models.Role, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, description, is_system, created_at, updated_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *RoleRepo) GetByName(ctx context.Context, name string) (*models.Role, error) {
	var role models.Role
	err := r.db.QueryRow(ctx, `SELECT id, name, description, is_system, created_at, updated_at FROM roles WHERE name = $1`, name).
		Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &role, err
}

func (r *RoleRepo) Create(ctx context.Context, name, description string) (*models.Role, error) {
	var role models.Role
	err := r.db.QueryRow(ctx, `
		INSERT INTO roles (name, description, is_system) VALUES ($1, $2, FALSE)
		RETURNING id, name, description, is_system, created_at, updated_at`, name, description).
		Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
	return &role, err
}

func (r *RoleRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM roles WHERE id = $1 AND is_system = FALSE`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("rôle système non supprimable ou introuvable")
	}
	return nil
}

func (r *RoleRepo) SetPermissions(ctx context.Context, roleID string, permissionKeys []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	if len(permissionKeys) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions WHERE key = ANY($2)`, roleID, permissionKeys); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *RoleRepo) PermissionsForRole(ctx context.Context, roleID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.key FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *RoleRepo) ListPermissions(ctx context.Context) ([]models.Permission, error) {
	rows, err := r.db.Query(ctx, `SELECT id, key, description FROM permissions ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []models.Permission
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(&p.ID, &p.Key, &p.Description); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}
