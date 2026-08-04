# ShareDesk — Architecture technique

Plateforme web de prise en main à distance auto-hébergée (alternative à AnyDesk/RustDesk).

## 1. Schéma des composants

```
                                   ┌─────────────────────────┐
                                   │        Frontend         │
                                   │   React + TS (SPA)      │
                                   │   client WebRTC + WS     │
                                   └────────────┬─────────────┘
                                                │ HTTPS (REST) + WSS (signaling)
                                                ▼
┌───────────────────────────────────────────────────────────────────────┐
│                              Backend (Go)                             │
│  ┌───────────────┐  ┌───────────────┐  ┌────────────────────────┐    │
│  │  API REST      │  │  Signaling WS │  │  Enrôlement / mTLS CA   │    │
│  │  auth, RBAC,   │  │  offer/answer │  │  interne (émission &   │    │
│  │  users, agents,│  │  ICE, commande│  │  révocation certificats │    │
│  │  logs, config  │  │  clavier/souris│  │  agents)               │    │
│  └───────┬────────┘  └───────┬───────┘  └────────────┬────────────┘    │
│          │                    │                        │              │
│          └────────────┬───────┴────────────────────────┘              │
│                        ▼                                              │
│              ┌──────────────────┐                                     │
│              │  Service crypto   │  (AES-256-GCM champs sensibles)     │
│              └─────────┬─────────┘                                     │
└────────────────────────┼───────────────────────────────────────────────┘
                          ▼
                 ┌──────────────────┐        ┌─────────────────┐
                 │   PostgreSQL      │        │   coturn (TURN/  │
                 │  (TLS, rôle appli │        │   STUN)          │
                 │   dédié, non-root)│        └────────┬─────────┘
                 └──────────────────┘                   │
                                                          │ relais média si NAT
                          ┌───────────────────────────────┘
                          ▼
                 ┌───────────────────────────┐
                 │   Agent Edge (Go)          │
                 │   Windows / Linux / macOS   │
                 │   - mTLS vers backend       │
                 │   - capture écran (DXGI/    │
                 │     X11/ScreenCaptureKit)   │
                 │   - encodage VP8/H.264      │
                 │   - injection clavier/souris│
                 │   - WebRTC (pion) DTLS/SRTP │
                 └───────────────────────────┘
```

Flux vidéo/audio : **P2P WebRTC** entre le navigateur (frontend) et l'agent quand possible, avec **coturn en relais (TURN)** dès que l'un des deux est derrière un NAT symétrique/pare-feu strict. Le backend n'est jamais dans le chemin du flux média (sauf mode relais applicatif futur type SFU si besoin de plusieurs viewers simultanés) — il ne fait que la **signalisation** (échange SDP/ICE) et le **contrôle** (auth, autorisation de session, clavier/souris si on choisit de les faire transiter par le canal data WebRTC plutôt que par le WS applicatif — voir §3).

## 2. Stack retenue et justification

| Composant | Choix | Justification |
|---|---|---|
| **Backend / API** | **Go** (net/http + chi ou gin, gorilla/websocket, pion/webrtc pour la partie signalisation/ICE) | Goroutines = coût mémoire très faible par connexion (~KB), adapté à des milliers de connexions WS simultanées (agents online en permanence + sessions actives). Compilation en binaire statique → image Docker minimale (scratch/distroless). `net/tls` gère nativement le mTLS (vérification de certificat client) sans dépendance externe, ce qui est central au modèle de sécurité anti-usurpation demandé. `pion/webrtc` est la lib WebRTC Go la plus mature, utile si on ajoute un mode SFU plus tard. |
| **Agent Edge** | **Go** (même langage que le backend) | Cross-compilation triviale vers Windows/Linux/macOS depuis une seule machine de build (`GOOS`/`GOARCH`), binaire natif sans runtime à installer, faible empreinte mémoire. Un seul langage pour backend+agent réduit la charge cognitive et permet de partager du code (types de messages signaling, client mTLS, crypto). `pion/webrtc` est réutilisé côté agent pour l'encodage/envoi du flux. Capture écran : `kbinani/screenshot` en fallback multiplateforme + bindings natifs dédiés pour la perf (DXGI Desktop Duplication sur Windows, X11 `xcbutil` sur Linux, `ScreenCaptureKit`/`CGDisplayStream` sur macOS). Injection clavier/souris : `robotgo` ou bindings natifs (SendInput sur Windows, XTest sur X11, CGEvent sur macOS). *Alternative envisagée : Rust pour l'agent (empreinte mémoire encore plus faible, pas de GC) — écarté en v1 pour rester sur un seul langage ; à réévaluer si la capture Go montre des limites de perf/latence en usage réel.* |
| **Streaming vidéo** | **WebRTC**, encodage **VP8 par défaut** (support natif large, bonne intégration pion), **H.264** en option si encodage matériel dispo côté agent (meilleure perf CPU, utile sur postes bas de gamme) | DTLS/SRTP natif au protocole → chiffrement du flux média sans configuration additionnelle. Faible latence adaptée au contrôle interactif temps réel. Compatible navigateur nativement côté frontend (pas de plugin). |
| **Frontend** | **React + TypeScript + Vite**, client WebRTC natif (`RTCPeerConnection`), Tailwind CSS pour l'UI, TanStack Query pour la couche API | Écosystème mature pour ce type d'app (dashboard + viewer temps réel), typage partagé avec le backend via schémas OpenAPI/JSON générés. |
| **Base de données** | **PostgreSQL 16**, migrations via **golang-migrate**, accès via **pgx v5** (requêtes explicites, repository pattern) — `sqlc` reste une évolution possible une fois le schéma stabilisé | Fiabilité, support JSONB pour métadonnées machine flexibles, TLS natif pour la connexion, rôle applicatif dédié à droits minimaux (pas de superuser). |
| **Chiffrement applicatif** | **AES-256-GCM** (lib `crypto/aes` + `crypto/cipher` Go), clé de chiffrement fournie via variable d'environnement / secret externe, jamais en dur | Chiffre en base : tokens d'agents, empreintes de certificats, métadonnées machine sensibles, secret LDAP (roadmap §6). |
| **Auth utilisateurs** | **Argon2id** (hash mot de passe), **JWT courte durée (15 min)** + **refresh token opaque rotatif** stocké hashé en base (révocable), **TOTP** (RFC 6238) pour MFA | Argon2id recommandé OWASP 2024+. Refresh token en base (et non JWT longue durée) permet la révocation immédiate — exigence explicite du cahier des charges. |
| **Auth agent ↔ serveur** | **mTLS** avec mini-CA interne au backend (ou intégration `cfssl`/`step-ca` en option), enrôlement par **token à usage unique** | Détaillé en §4. |
| **TURN/STUN** | **coturn** (image officielle), credentials générés dynamiquement (auth REST éphémère via secret partagé, pas de compte statique) | Standard de facto, éprouvé, bien supporté par pion et les navigateurs. |
| **Conteneurisation** | **Docker Compose** — services `backend`, `frontend`, `postgres`, `coturn` | Pas de reverse proxy dans le périmètre (TLS géré en dehors) conformément à la demande. |
| **CI/CD** | **GitHub Actions** → build multi-stage → push `ghcr.io/hyugo22/sharedesk-backend`, `ghcr.io/hyugo22/sharedesk-frontend` (tags `sha`, `latest`, `vX.Y.Z`) | ⚠️ **Point d'attention** : GHCr exige des noms d'image en minuscules. Le repo s'appelle `shareDesk` (casse mixte) — les images seront nommées en minuscule (`sharedesk-backend`, etc.), ce qui est indépendant de la casse du nom du repo GitHub lui-même. |

## 3. Canal clavier/souris

Deux options possibles :
- **Via le DataChannel WebRTC** (canal fiable ordonné) : cohérent avec le flux vidéo, faible latence, chiffré nativement (DTLS). **Recommandé**.
- Via le WebSocket de signalisation applicatif (plus simple à débugger mais latence/charge supplémentaire sur le backend qui devient alors dans le chemin critique).

→ On retient le **DataChannel WebRTC** pour les événements clavier/souris une fois la session établie ; le WebSocket applicatif ne sert qu'à la **signalisation** (offer/answer/ICE) et aux **commandes de contrôle hors média** (démarrage/fin de session, statut agent, révocation).

## 4. Modèle anti-usurpation (mTLS)

1. Un admin/tech génère un **token d'enrôlement à usage unique et à durée limitée** depuis l'interface.
2. L'agent, au premier lancement, génère **localement** une paire de clés (la clé privée ne quitte jamais la machine), crée une CSR, et l'envoie au backend avec le token d'enrôlement (via TLS serveur classique, avant d'avoir son propre certificat).
3. Le backend (mini-CA interne) signe la CSR et retourne un **certificat client** à l'agent + le certificat/empreinte de la CA serveur à épingler (**certificate pinning**, pas de confiance au premier contact au-delà de cet échange initial contrôlé par le token).
4. Toute connexion ultérieure agent→serveur est en **mTLS strict** : le serveur vérifie le certificat client contre sa CA + une **liste de révocation (CRL) vérifiée à chaque connexion**, l'agent vérifie le certificat serveur contre l'empreinte épinglée.
5. Révocation d'un agent = ajout à la CRL côté backend → prochaine tentative de connexion rejetée immédiatement, session active coupée.
6. Les mises à jour de l'agent sont **signées** (clé de signature séparée de la CA mTLS) ; l'agent vérifie la signature avant d'appliquer un binaire.

## 5. Structure de repo proposée

```
sharedesk/
├── backend/          # API Go, signaling WS, mini-CA, service crypto
├── frontend/          # React + TS
├── agent/             # Agent Go (capture, injection, client WebRTC/mTLS)
├── infra/
│   ├── docker-compose.yml
│   ├── docker-compose.override.yml
│   └── .github/workflows/   # ou à la racine .github/workflows
├── docs/
│   ├── ARCHITECTURE.md
│   ├── SECURITY.md
│   └── DATA_MODEL.md
├── .env.example
└── README.md
```

## 6. Point d'extension AD/LDAP (roadmap)

Prévu dès la v1 comme **interface Go** (`internal/directory.Provider`) non implémentée mais définie, avec un stub "local" (auth par mot de passe uniquement). L'implémentation LDAP viendra en dernier, strictement en lecture (`search`/`bind` uniquement — jamais `add`/`modify`/`delete`), configuration pilotée depuis l'UI et stockée chiffrée en base (mot de passe du compte de service via le service crypto AES-256-GCM déjà en place pour les tokens agents).
