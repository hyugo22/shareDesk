// Package repository implémente l'accès aux données via pgx (sans ORM).
package repository

import "github.com/jackc/pgx/v5/pgxpool"

type Repositories struct {
	Users       *UserRepo
	Roles       *RoleRepo
	Agents      *AgentRepo
	Enrollment  *EnrollmentRepo
	Sessions    *SessionRepo
	Audit       *AuditRepo
	Settings    *SettingsRepo
	LDAPConfig  *LDAPConfigRepo
}

func New(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		Users:      &UserRepo{db: pool},
		Roles:      &RoleRepo{db: pool},
		Agents:     &AgentRepo{db: pool},
		Enrollment: &EnrollmentRepo{db: pool},
		Sessions:   &SessionRepo{db: pool},
		Audit:      &AuditRepo{db: pool},
		Settings:   &SettingsRepo{db: pool},
		LDAPConfig: &LDAPConfigRepo{db: pool},
	}
}
