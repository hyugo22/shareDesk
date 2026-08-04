# Sécurité — modèle de menace, secrets, logs

## 1. Modèle de menace (résumé)

| Acteur / scénario | Risque | Mitigation en place |
|---|---|---|
| Attaquant réseau externe, sans compte | Accès non autorisé à l'API, brute force login | TLS obligatoire (à la charge du déploiement, voir §2), rate limiting sur `/auth/login` et `/agents/enroll` ([internal/ratelimit](../backend/internal/ratelimit)), verrouillage de compte après 5 échecs (15 min) |
| Attaquant réseau, MITM entre agent et serveur | Interception/usurpation du serveur, injection de commandes de contrôle sur un poste | **mTLS strict** : l'agent épingle le certificat de la CA reçu à l'enrôlement ([agent/internal/identity](../agent/internal/identity)) et ne fait plus jamais confiance à un autre émetteur ; le serveur vérifie le certificat client de l'agent (`tls.RequireAndVerifyClientCert`, [backend/cmd/server/main.go](../backend/cmd/server/main.go)) |
| Poste agent compromis, tentative de réutiliser son identité ailleurs | Usurpation d'un agent légitime | Chaque agent a une paire de clés générée **localement** à l'enrôlement (clé privée jamais transmise) ; le certificat est lié à un `cert_serial`/`cert_fingerprint_sha256` uniques en base |
| Poste agent compromis, détecté a posteriori | Maintien d'accès malgré la compromission | Révocation immédiate depuis l'UI admin : ajout à la CRL applicative (`agent_cert_revocations`), vérifiée à **chaque** connexion mTLS ([internal/httpapi/agent_channel.go](../backend/internal/httpapi/agent_channel.go)), déconnexion forcée de la session WS en cours |
| Utilisateur authentifié, actions hors de son rôle | Élévation de privilèges horizontale/verticale | RBAC appliqué par middleware sur chaque route ([internal/httpapi/middleware.go](../backend/internal/httpapi/middleware.go)), permissions dérivées des tables `roles`/`permissions`, pas de rôle codé en dur dans les handlers |
| Compromission de la base de données (dump) | Exposition de secrets stockés | Mots de passe en Argon2id (jamais réversibles), refresh tokens stockés en hash SHA-256 uniquement, secrets sensibles (TOTP, mot de passe LDAP) chiffrés en AES-256-GCM ([internal/crypto](../backend/internal/crypto)) avec clé hors base (`APP_ENCRYPTION_KEY`) |
| Session de contrôle à distance détournée | Accès à un poste sans autorisation | Session liée à un JWT court (15 min) + à un `session_id` vérifié côté serveur avant tout relais de signalisation ; toute session est tracée (début/fin, utilisateur, agent, IP) dans `control_sessions` et les logs d'audit |
| Mise à jour d'agent malveillante | Compromission de flotte via un binaire piégé | **Point d'attention non résolu en v1** : la vérification de signature des binaires/mises à jour d'agent (exigée au cahier des charges) n'est pas encore implémentée — voir §4 limitations |

## 2. Chiffrement en transit

- **API/WebSocket navigateur ↔ backend** : TLS attendu en production. Le binaire backend peut servir en TLS natif (certificats fournis via montage de volume) ou être placé derrière une terminaison TLS gérée en dehors du périmètre de ce projet (cf. cahier des charges : pas de reverse proxy fourni ici). En dev local (`docker-compose.override.yml`), le trafic interne au réseau Docker est en clair — ne pas utiliser tel quel en production.
- **Agent ↔ backend** : mTLS **toujours** actif, y compris en développement (listener dédié `BACKEND_MTLS_ADDR`, indépendant de la terminaison TLS publique).
- **Base de données** : connexion chiffrée par défaut (`POSTGRES_SSLMODE=require`), désactivée uniquement dans l'override de dev local.
- **WebRTC** : DTLS/SRTP natif au protocole, non désactivable.

## 3. Gestion des secrets

- Tous les secrets (mots de passe DB, `JWT_SIGNING_SECRET`, `APP_ENCRYPTION_KEY`, `CA_KEY_PASSPHRASE`, `TURN_SHARED_SECRET`) sont fournis via variables d'environnement (`.env`, non commité — voir `.env.example`).
- Aucun secret n'est codé en dur dans le code source.
- `APP_ENCRYPTION_KEY` chiffre en base : secrets TOTP, mot de passe du compte de service LDAP. Sa perte rend ces valeurs irrécupérables (à sauvegarder au même niveau que la base).
- `CA_KEY_PASSPHRASE` protège la clé privée de la CA interne (stockée chiffrée sur le volume `ca_data`). Sa perte, combinée à celle du volume, oblige à régénérer la CA — ce qui **révoque de facto tous les agents enrôlés** (ils devront être ré-enrôlés).
- Le mot de passe du compte de service LDAP n'est jamais journalisé ni renvoyé en clair par l'API une fois enregistré (voir [handlers_ldap.go](../backend/internal/httpapi/handlers_ldap.go)).
- En CI/CD, seul `GITHUB_TOKEN` (fourni automatiquement, scope `packages:write`) est nécessaire pour publier sur ghcr.io. Le déploiement SSH optionnel utilise des secrets de repo dédiés (`DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY`, `DEPLOY_PATH`), jamais commités.

## 4. Politique de logs d'audit

- Table `audit_logs` **append-only** : des triggers PostgreSQL rejettent explicitement tout `UPDATE`/`DELETE`, y compris via le rôle applicatif ([migration 0001](../backend/migrations/0001_init.up.sql)).
- Actions journalisées : connexion/échec de connexion, activation MFA, création/modification de compte ou de rôle, changement de permission, enrôlement/révocation d'agent, début/fin de session de contrôle, modification de paramètre (y compris configuration LDAP — sans jamais journaliser le mot de passe), tests de connexion LDAP.
- Chaque entrée porte : horodatage, type d'acteur (utilisateur/agent/système), identifiant de l'acteur, action, cible, **adresse IP source**, détails structurés (JSONB).
- Consultation restreinte à la permission `audit.read` (rôles admin/tech par défaut), export CSV/JSON disponible depuis l'UI.
- **Non implémenté en v1** : rotation/purge automatique des logs (rétention illimitée par défaut), export vers un SIEM externe.

## 5. Limitations connues de la v1 (à traiter avant une mise en production)

- **Signature des binaires agent** : non implémentée. À ajouter avant tout mécanisme de mise à jour automatique des agents (signature par une clé séparée de la CA mTLS, vérifiée par l'agent avant application).
- **Flux vidéo** : la session de contrôle transmet des captures JPEG basse fréquence sur un DataChannel WebRTC réel, et non une piste vidéo WebRTC encodée (VP8/H.264) — voir [agent/internal/rtcsession](../agent/internal/rtcsession). Fonctionnel de bout en bout, mais pas la fluidité temps réel visée à terme.
- **Injection clavier/souris** : implémentée nativement pour Windows uniquement (`SendInput`, non testée sur un poste Windows réel à ce stade — voir avertissement dans le rapport de session). Linux (XTest) et macOS (CGEvent) sont de purs points d'extension non implémentés.
- **Refresh token côté navigateur** : stocké en `localStorage` pour ce squelette v1 (durcissement ultérieur recommandé : cookie `httpOnly` + `SameSite=Strict`).
- **Credentials TURN** : secret partagé statique (`TURN_SHARED_SECRET`) passé à coturn et à l'agent via configuration ; la génération de credentials éphémères via l'API backend (REST API coturn) est un point d'amélioration identifié mais non implémenté.
- **Rate limiting** : en mémoire par instance ([internal/ratelimit](../backend/internal/ratelimit)), ne survit pas à un redémarrage et ne se coordonne pas entre plusieurs instances backend (pas de scénario multi-instance prévu en v1).

## 6. Intégration AD/LDAP — contrainte de sécurité

Voir [internal/directory/provider.go](../backend/internal/directory/provider.go) : toute implémentation future de `Provider` doit se limiter strictement aux opérations LDAP `search`/`bind`. Aucune opération `add`/`modify`/`delete` ne doit jamais être effectuée côté annuaire. Le compte de service configuré doit être un compte à droits de lecture seule côté AD — cette exigence doit être vérifiée côté annuaire par l'équipe qui déploie l'intégration, ce projet ne peut techniquement pas l'imposer depuis le code client LDAP.
