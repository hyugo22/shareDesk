// Package directory est le point d'extension pour l'intégration à un
// annuaire externe (Active Directory / LDAP — voir docs/ARCHITECTURE.md §6).
//
// CONTRAINTE DE SÉCURITÉ IMPÉRATIVE : toute implémentation de Provider DOIT
// être strictement en lecture vis-à-vis de l'annuaire. Seules les opérations
// LDAP `search` et `bind` sont autorisées. Aucune implémentation ne doit
// jamais effectuer d'opération `add`, `modify`, ou `delete` sur l'annuaire.
// Le compte de service configuré côté AD doit lui-même être un compte à
// droits de lecture seule (voir docs/SECURITY.md).
//
// La v1 n'embarque volontairement pas de client LDAP réel : seul ce point
// d'extension est défini, avec une implémentation "local" qui désactive
// l'intégration. L'implémentation LDAP viendra en dernier dans la roadmap.
package directory

import (
	"context"
	"errors"
)

type Config struct {
	Host             string
	Port             int
	ConnectionMode   string // "ldaps" | "starttls" — jamais LDAP en clair
	BindDN           string
	BindPassword     string // déchiffré en mémoire uniquement, jamais journalisé
	BaseDN           string
	AttributeMapping map[string]string
	GroupRoleMapping map[string]string
}

type Identity struct {
	Username string
	Email    string
	Groups   []string
}

type SyncResult struct {
	UsersFound    int
	UsersSynced   int
	GroupsMapped  int
	Warnings      []string
}

type Provider interface {
	// TestConnection effectue un bind de test en lecture seule et retourne une
	// erreur explicite en cas d'échec, sans jamais exposer le mot de passe.
	TestConnection(ctx context.Context, cfg Config) error

	// Authenticate vérifie les identifiants d'un utilisateur via un bind LDAP
	// (jamais de comparaison de hash local pour les comptes fédérés AD).
	Authenticate(ctx context.Context, cfg Config, username, password string) (*Identity, error)

	// SyncUsers interroge l'annuaire (search uniquement) et retourne les
	// identités/groupes à mapper vers les rôles internes.
	SyncUsers(ctx context.Context, cfg Config) (*SyncResult, error)
}

var ErrNotImplemented = errors.New("intégration AD/LDAP non activée dans cette version")

// LocalProvider est l'implémentation par défaut : aucune intégration externe.
// Remplacée par une implémentation LDAP réelle (roadmap) sans changement des
// appelants (internal/httpapi/handlers_ldap.go), qui ne dépendent que de Provider.
type LocalProvider struct{}

func (LocalProvider) TestConnection(ctx context.Context, cfg Config) error { return ErrNotImplemented }

func (LocalProvider) Authenticate(ctx context.Context, cfg Config, username, password string) (*Identity, error) {
	return nil, ErrNotImplemented
}

func (LocalProvider) SyncUsers(ctx context.Context, cfg Config) (*SyncResult, error) {
	return nil, ErrNotImplemented
}
