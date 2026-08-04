# Modèle de données

Schéma complet : [`backend/migrations/0001_init.up.sql`](../backend/migrations/0001_init.up.sql). Migrations gérées via [golang-migrate](https://github.com/golang-migrate/migrate).

## Tables principales

- **roles / permissions / role_permissions** — RBAC extensible (pas figé à deux rôles). Rôles système de départ : `admin`, `tech`, `user` (voir §5 du cahier des charges), modifiables/complétables depuis l'UI.
- **users** — mot de passe en Argon2id, secret TOTP chiffré (`mfa_totp_secret_enc`, AES-256-GCM), verrouillage de compte (`failed_login_attempts`, `locked_until`).
- **refresh_tokens** — uniquement le hash SHA-256 du token est stocké → révocation immédiate possible, jamais de token en clair en base.
- **agents** — identité liée à un certificat mTLS (`cert_serial`, `cert_fingerprint_sha256`, uniques), pas de secret partagé statique.
- **enrollment_tokens** — tokens d'enrôlement à usage unique et à durée limitée, hashés en base.
- **agent_cert_revocations** — CRL applicative, vérifiée à chaque connexion mTLS d'un agent.
- **control_sessions** — traçabilité des sessions de contrôle à distance (qui, quelle machine, début/fin).
- **audit_logs** — append-only : des triggers PostgreSQL rejettent tout `UPDATE`/`DELETE`, même via le rôle applicatif.
- **app_settings** — paramètres généraux de l'instance (clé/valeur JSONB), modifiables depuis l'UI Administration.
- **ldap_config** — point d'extension AD/LDAP (§6 du cahier des charges) : ligne unique, mot de passe du compte de service chiffré, jamais renvoyé en clair par l'API. Compte de service **strictement lecture seule** côté annuaire (voir commentaire sur la table et [docs/SECURITY.md](SECURITY.md)).

## Rôle applicatif PostgreSQL

Le backend se connecte avec un rôle dédié à droits minimaux (`sharedesk_app`), sans droits `SUPERUSER`/`CREATEDB`, limité aux tables ci-dessus. Aucune connexion applicative avec le compte superutilisateur PostgreSQL.
