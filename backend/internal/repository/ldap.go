package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyugo22/sharedesk/backend/internal/models"
)

type LDAPConfigRepo struct{ db *pgxpool.Pool }

// Get retourne la configuration LDAP (ligne unique, créée implicitement au premier accès).
func (r *LDAPConfigRepo) Get(ctx context.Context) (*models.LDAPConfig, error) {
	var c models.LDAPConfig
	var attrRaw, groupRaw []byte
	var bindPassEnc *string
	err := r.db.QueryRow(ctx, `
		SELECT enabled, host, port, connection_mode, bind_dn, bind_password_enc, base_dn,
		       attribute_mapping, group_role_mapping, sync_interval_minutes, last_sync_at,
		       coalesce(last_sync_status,''), updated_at
		FROM ldap_config WHERE id = TRUE`).Scan(
		&c.Enabled, &c.Host, &c.Port, &c.ConnectionMode, &c.BindDN, &bindPassEnc, &c.BaseDN,
		&attrRaw, &groupRaw, &c.SyncIntervalMinutes, &c.LastSyncAt, &c.LastSyncStatus, &c.UpdatedAt)
	if err != nil {
		// Pas encore de ligne : configuration par défaut désactivée.
		return &models.LDAPConfig{Enabled: false, Port: 636, ConnectionMode: "ldaps"}, nil
	}
	if bindPassEnc != nil && *bindPassEnc != "" {
		c.BindPasswordEnc = *bindPassEnc
		c.HasBindPassword = true
	}
	_ = json.Unmarshal(attrRaw, &c.AttributeMapping)
	_ = json.Unmarshal(groupRaw, &c.GroupRoleMapping)
	return &c, nil
}

// Upsert écrit la configuration. bindPasswordEnc doit déjà être chiffré (voir internal/crypto) ;
// passer nil pour conserver le mot de passe existant inchangé.
func (r *LDAPConfigRepo) Upsert(ctx context.Context, c *models.LDAPConfig, bindPasswordEnc *string, updatedBy string) error {
	attrRaw, err := json.Marshal(c.AttributeMapping)
	if err != nil {
		return err
	}
	groupRaw, err := json.Marshal(c.GroupRoleMapping)
	if err != nil {
		return err
	}

	if bindPasswordEnc != nil {
		_, err = r.db.Exec(ctx, `
			INSERT INTO ldap_config (id, enabled, host, port, connection_mode, bind_dn, bind_password_enc,
			                          base_dn, attribute_mapping, group_role_mapping, sync_interval_minutes, updated_by)
			VALUES (TRUE,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (id) DO UPDATE SET
				enabled=$1, host=$2, port=$3, connection_mode=$4, bind_dn=$5, bind_password_enc=$6,
				base_dn=$7, attribute_mapping=$8, group_role_mapping=$9, sync_interval_minutes=$10,
				updated_by=$11, updated_at=now()`,
			c.Enabled, c.Host, c.Port, c.ConnectionMode, c.BindDN, *bindPasswordEnc,
			c.BaseDN, attrRaw, groupRaw, c.SyncIntervalMinutes, nullIfEmpty(updatedBy))
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO ldap_config (id, enabled, host, port, connection_mode, bind_dn,
		                          base_dn, attribute_mapping, group_role_mapping, sync_interval_minutes, updated_by)
		VALUES (TRUE,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			enabled=$1, host=$2, port=$3, connection_mode=$4, bind_dn=$5,
			base_dn=$6, attribute_mapping=$7, group_role_mapping=$8, sync_interval_minutes=$9,
			updated_by=$10, updated_at=now()`,
		c.Enabled, c.Host, c.Port, c.ConnectionMode, c.BindDN,
		c.BaseDN, attrRaw, groupRaw, c.SyncIntervalMinutes, nullIfEmpty(updatedBy))
	return err
}

func (r *LDAPConfigRepo) RecordSyncResult(ctx context.Context, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE ldap_config SET last_sync_at = now(), last_sync_status = $1 WHERE id = TRUE`, status)
	return err
}
