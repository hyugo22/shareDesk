# ShareDesk

Plateforme web de prise en main à distance auto-hébergée (alternative à AnyDesk/RustDesk) : serveur central, interface web d'administration/contrôle, et agent léger multiplateforme.

## Structure du repo

- [`backend/`](backend/) — API Go (auth, RBAC, signalisation WebSocket, mini-CA mTLS, audit)
- [`frontend/`](frontend/) — Interface web React + TypeScript
- [`agent/`](agent/) — Agent Edge Go (Windows/Linux/macOS)
- [`infra/`](infra/) — Docker Compose (dev/prod) et configuration coturn
- [`docs/`](docs/) — Architecture, sécurité, modèle de données

Voir [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) pour le détail des choix techniques et [docs/SECURITY.md](docs/SECURITY.md) pour le modèle de menace.

## Démarrage rapide (dev)

```bash
cp .env.example .env   # renseigner les secrets
cd infra
docker compose up --build
```

## Statut

Scaffold, auth/RBAC, enrôlement d'agent (mTLS), session de contrôle basique (WebRTC) et pipeline CI/CD sont en place et ont été testés de bout en bout via `docker compose up`. Voir [docs/SECURITY.md](docs/SECURITY.md) §5 pour les limitations connues de cette v1. L'intégration AD/LDAP reste au stade de point d'extension (non implémentée) — dernière étape de la roadmap.
