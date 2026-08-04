-- Extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

-- ---------------------------------------------------------------------------
-- RBAC : rôles et permissions (modèle extensible, pas figé à admin/tech)
-- ---------------------------------------------------------------------------
CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    is_system   BOOLEAN NOT NULL DEFAULT FALSE, -- rôles fournis par défaut (admin/tech), non supprimables
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key         TEXT NOT NULL UNIQUE, -- ex: "users.manage", "agents.control", "audit.read"
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- ---------------------------------------------------------------------------
-- Utilisateurs
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                  CITEXT NOT NULL UNIQUE,
    display_name           TEXT NOT NULL,
    password_hash          TEXT NOT NULL, -- Argon2id
    role_id                UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    mfa_enabled            BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_totp_secret_enc    TEXT,           -- chiffré AES-256-GCM (jamais en clair)
    is_active              BOOLEAN NOT NULL DEFAULT TRUE,
    failed_login_attempts  INT NOT NULL DEFAULT 0,
    locked_until           TIMESTAMPTZ,
    last_login_at          TIMESTAMPTZ,
    last_login_ip          INET,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE, -- SHA-256 du refresh token opaque (jamais le token en clair)
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    user_agent  TEXT,
    ip_address  INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);

-- ---------------------------------------------------------------------------
-- Agents & enrôlement (mTLS)
-- ---------------------------------------------------------------------------
CREATE TABLE enrollment_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash      TEXT NOT NULL UNIQUE, -- SHA-256 du token à usage unique
    description     TEXT NOT NULL DEFAULT '',
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ,
    used_by_agent_id UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agents (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    TEXT NOT NULL,
    hostname                TEXT NOT NULL,
    os                      TEXT NOT NULL,
    os_version              TEXT NOT NULL DEFAULT '',
    arch                    TEXT NOT NULL DEFAULT '',
    agent_version           TEXT NOT NULL DEFAULT '',
    tags                    TEXT[] NOT NULL DEFAULT '{}',
    status                  TEXT NOT NULL DEFAULT 'offline' CHECK (status IN ('online','offline')),
    cert_serial             TEXT NOT NULL UNIQUE,   -- numéro de série du certificat client mTLS émis
    cert_fingerprint_sha256 TEXT NOT NULL UNIQUE,    -- empreinte du certificat, vérifiée à chaque connexion
    enrollment_token_id     UUID REFERENCES enrollment_tokens(id) ON DELETE SET NULL,
    enrolled_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at            TIMESTAMPTZ,
    revoked_at              TIMESTAMPTZ,
    revoked_reason          TEXT,
    metadata                JSONB NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_tags ON agents USING GIN(tags);

ALTER TABLE enrollment_tokens
    ADD CONSTRAINT fk_enrollment_tokens_agent
    FOREIGN KEY (used_by_agent_id) REFERENCES agents(id) ON DELETE SET NULL;

-- Liste de révocation de certificats — vérifiée à chaque connexion mTLS d'un agent.
CREATE TABLE agent_cert_revocations (
    cert_serial TEXT PRIMARY KEY,
    agent_id    UUID REFERENCES agents(id) ON DELETE CASCADE,
    revoked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason      TEXT NOT NULL DEFAULT ''
);

-- ---------------------------------------------------------------------------
-- Sessions de contrôle à distance
-- ---------------------------------------------------------------------------
CREATE TABLE control_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended','failed')),
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    ip_address  INET,
    metadata    JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_control_sessions_agent ON control_sessions(agent_id);
CREATE INDEX idx_control_sessions_user ON control_sessions(user_id);

-- ---------------------------------------------------------------------------
-- Logs d'audit — append-only (immuable), consultable par rôles autorisés
-- ---------------------------------------------------------------------------
CREATE TABLE audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_type      TEXT NOT NULL CHECK (actor_type IN ('user','agent','system')),
    actor_user_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_agent_id  UUID REFERENCES agents(id) ON DELETE SET NULL,
    action          TEXT NOT NULL, -- ex: "auth.login.success", "agent.revoke", "settings.update"
    target_type     TEXT,
    target_id       TEXT,
    ip_address      INET,
    details         JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_audit_logs_occurred_at ON audit_logs(occurred_at DESC);
CREATE INDEX idx_audit_logs_actor_user ON audit_logs(actor_user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);

-- Applique l'immuabilité au niveau base : aucune modification/suppression possible,
-- même par erreur applicative ou accès direct malveillant avec le rôle applicatif.
CREATE OR REPLACE FUNCTION reject_audit_log_mutation() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs est append-only : UPDATE/DELETE interdits';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation();

CREATE TRIGGER trg_audit_logs_no_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation();

-- ---------------------------------------------------------------------------
-- Paramètres applicatifs (section Administration)
-- ---------------------------------------------------------------------------
CREATE TABLE app_settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- ---------------------------------------------------------------------------
-- Intégration AD/LDAP (point d'extension roadmap — lecture seule stricte)
-- ---------------------------------------------------------------------------
CREATE TABLE ldap_config (
    id                  BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id = TRUE), -- ligne unique
    enabled             BOOLEAN NOT NULL DEFAULT FALSE,
    host                TEXT NOT NULL DEFAULT '',
    port                INT NOT NULL DEFAULT 636,
    connection_mode     TEXT NOT NULL DEFAULT 'ldaps' CHECK (connection_mode IN ('ldaps','starttls')),
    bind_dn             TEXT NOT NULL DEFAULT '',
    bind_password_enc   TEXT, -- chiffré AES-256-GCM, jamais en clair, jamais renvoyé par l'API
    base_dn             TEXT NOT NULL DEFAULT '',
    attribute_mapping   JSONB NOT NULL DEFAULT '{"username":"sAMAccountName","email":"mail","groups":"memberOf"}',
    group_role_mapping  JSONB NOT NULL DEFAULT '{}', -- { "CN=IT-Admins,OU=...": "admin", ... }
    sync_interval_minutes INT NOT NULL DEFAULT 60,
    last_sync_at        TIMESTAMPTZ,
    last_sync_status     TEXT,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by           UUID REFERENCES users(id) ON DELETE SET NULL
);
COMMENT ON TABLE ldap_config IS
  'Compte de service configuré ici DOIT être en lecture seule côté AD (search/bind uniquement). '
  'L''application ne doit jamais effectuer d''opération LDAP add/modify/delete.';

-- ---------------------------------------------------------------------------
-- Données de départ : rôles système + permissions de base
-- ---------------------------------------------------------------------------
INSERT INTO roles (name, description, is_system) VALUES
    ('admin', 'Accès total à la plateforme', TRUE),
    ('tech',  'Gestion des agents, sessions et paramètres, sans gestion des comptes/rôles', TRUE),
    ('user',  'Peut uniquement initier des sessions sur les machines qui lui sont assignées', TRUE);

INSERT INTO permissions (key, description) VALUES
    ('users.manage',      'Créer/modifier/supprimer des utilisateurs'),
    ('roles.manage',      'Créer/modifier/supprimer des rôles et permissions'),
    ('agents.manage',     'Gérer les agents (enrôlement, révocation, mise à jour)'),
    ('agents.control',    'Initier une session de contrôle à distance'),
    ('settings.manage',   'Modifier les paramètres généraux et techniques de l''application'),
    ('audit.read',        'Consulter les logs d''audit'),
    ('ldap.manage',       'Configurer l''intégration AD/LDAP');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'admin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'tech' AND p.key IN ('agents.manage','agents.control','settings.manage','audit.read');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.key IN ('agents.control');
