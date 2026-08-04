// Package models définit les entités du domaine, reflet des tables SQL.
package models

import "time"

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Permission struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
}

type User struct {
	ID                 string     `json:"id"`
	Email              string     `json:"email"`
	DisplayName        string     `json:"display_name"`
	PasswordHash       string     `json:"-"`
	RoleID             string     `json:"role_id"`
	RoleName           string     `json:"role_name,omitempty"`
	MFAEnabled         bool       `json:"mfa_enabled"`
	MFATOTPSecretEnc   string     `json:"-"`
	IsActive           bool       `json:"is_active"`
	FailedLoginAttempts int       `json:"-"`
	LockedUntil        *time.Time `json:"locked_until,omitempty"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP        string     `json:"last_login_ip,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type Agent struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	Hostname              string     `json:"hostname"`
	OS                    string     `json:"os"`
	OSVersion             string     `json:"os_version"`
	Arch                  string     `json:"arch"`
	AgentVersion          string     `json:"agent_version"`
	Tags                  []string   `json:"tags"`
	Status                string     `json:"status"`
	CertSerial            string     `json:"cert_serial"`
	CertFingerprintSHA256 string     `json:"cert_fingerprint_sha256"`
	EnrolledAt            time.Time  `json:"enrolled_at"`
	LastSeenAt            *time.Time `json:"last_seen_at,omitempty"`
	RevokedAt             *time.Time `json:"revoked_at,omitempty"`
	RevokedReason         string     `json:"revoked_reason,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type EnrollmentToken struct {
	ID           string     `json:"id"`
	Description  string     `json:"description"`
	CreatedBy    string     `json:"created_by"`
	ExpiresAt    time.Time  `json:"expires_at"`
	UsedAt       *time.Time `json:"used_at,omitempty"`
	UsedByAgent  string     `json:"used_by_agent_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type ControlSession struct {
	ID        string     `json:"id"`
	AgentID   string     `json:"agent_id"`
	UserID    string     `json:"user_id"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	IPAddress string     `json:"ip_address,omitempty"`
}

type AuditLog struct {
	ID           int64          `json:"id"`
	OccurredAt   time.Time      `json:"occurred_at"`
	ActorType    string         `json:"actor_type"`
	ActorUserID  *string        `json:"actor_user_id,omitempty"`
	ActorAgentID *string        `json:"actor_agent_id,omitempty"`
	Action       string         `json:"action"`
	TargetType   string         `json:"target_type,omitempty"`
	TargetID     string         `json:"target_id,omitempty"`
	IPAddress    string         `json:"ip_address,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

// LDAPConfig est le point d'extension AD/LDAP (roadmap, lecture seule stricte).
// BindPasswordEnc est chiffré (AES-256-GCM) et n'est jamais exposé en clair par l'API.
type LDAPConfig struct {
	Enabled              bool           `json:"enabled"`
	Host                 string         `json:"host"`
	Port                 int            `json:"port"`
	ConnectionMode       string         `json:"connection_mode"` // "ldaps" | "starttls"
	BindDN               string         `json:"bind_dn"`
	BindPasswordEnc      string         `json:"-"`
	HasBindPassword      bool           `json:"has_bind_password"`
	BaseDN               string         `json:"base_dn"`
	AttributeMapping     map[string]any `json:"attribute_mapping"`
	GroupRoleMapping     map[string]any `json:"group_role_mapping"`
	SyncIntervalMinutes  int            `json:"sync_interval_minutes"`
	LastSyncAt           *time.Time     `json:"last_sync_at,omitempty"`
	LastSyncStatus       string         `json:"last_sync_status,omitempty"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

// Permission keys connues — voir migration 0001_init pour la définition en base.
const (
	PermUsersManage    = "users.manage"
	PermRolesManage    = "roles.manage"
	PermAgentsManage   = "agents.manage"
	PermAgentsControl  = "agents.control"
	PermSettingsManage = "settings.manage"
	PermAuditRead      = "audit.read"
	PermLDAPManage     = "ldap.manage"
)
