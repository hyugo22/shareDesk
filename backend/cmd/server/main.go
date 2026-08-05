// Commande server démarre le backend ShareDesk : API REST + WebSocket de
// signalisation (port BACKEND_HTTP_ADDR) et listener mTLS dédié aux agents
// (port BACKEND_MTLS_ADDR).
package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hyugo22/sharedesk/backend/internal/auth"
	"github.com/hyugo22/sharedesk/backend/internal/ca"
	"github.com/hyugo22/sharedesk/backend/internal/config"
	appcrypto "github.com/hyugo22/sharedesk/backend/internal/crypto"
	"github.com/hyugo22/sharedesk/backend/internal/db"
	"github.com/hyugo22/sharedesk/backend/internal/httpapi"
	"github.com/hyugo22/sharedesk/backend/internal/repository"
	"github.com/hyugo22/sharedesk/backend/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}

	if err := db.RunMigrations(cfg.PostgresDSN); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("connexion base de données: %v", err)
	}
	defer pool.Close()

	repos := repository.New(pool)

	// Le premier compte administrateur est créé depuis l'assistant de
	// configuration initiale du frontend (GET/POST /api/v1/setup), pas ici :
	// voir internal/httpapi/handlers_setup.go.

	sealer, err := appcrypto.NewSealer(cfg.AppEncryptionKey)
	if err != nil {
		log.Fatalf("chiffrement applicatif: %v", err)
	}

	caDataDir := getEnv("CA_DATA_DIR", "/data/ca")
	internalCA, err := ca.LoadOrCreate(caDataDir, cfg.CAKeyPassphrase)
	if err != nil {
		log.Fatalf("autorité de certification interne: %v", err)
	}

	jwtIssuer := auth.NewJWTIssuer(cfg.JWTSigningSecret, cfg.AccessTokenTTL)

	hub := ws.NewHub(func(agentID string, online bool) {
		status := "offline"
		if online {
			status = "online"
		}
		if err := repos.Agents.SetStatus(context.Background(), agentID, status); err != nil {
			log.Printf("mise à jour statut agent %s: %v", agentID, err)
		}
	})

	server := httpapi.NewServer(repos, jwtIssuer, sealer, internalCA, hub, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, cfg.CORSAllowedOrigins, cfg.DownloadsDir)

	go staleAgentSweeper(repos)

	// --- Listener API/WS (frontend + utilisateurs) ---
	apiSrv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      server.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // WebSocket de longue durée
	}
	go func() {
		log.Printf("API démarrée sur %s", cfg.HTTPAddr)
		if err := apiSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serveur API: %v", err)
		}
	}()

	// --- Listener mTLS dédié aux agents ---
	// Authentification mutuelle stricte : le serveur exige et vérifie un
	// certificat client signé par la CA interne, et rejette toute connexion
	// dont le certificat figure dans la CRL applicative (agent_cert_revocations),
	// vérifiée à chaque poignée de main TLS.
	mtlsAddr := getEnv("BACKEND_MTLS_ADDR", ":8443")
	agentHostnames := strings.Split(getEnv("BACKEND_AGENT_HOSTNAMES", "localhost"), ",")
	serverCertPEM, serverKeyPEM, err := internalCA.IssueServerCert(agentHostnames)
	if err != nil {
		log.Fatalf("émission certificat serveur mTLS: %v", err)
	}
	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		log.Fatalf("chargement certificat serveur mTLS: %v", err)
	}

	// La révocation (CRL applicative en base) est vérifiée dans handleAgentChannel
	// juste après la poignée de main TLS, avant tout traitement applicatif.
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    internalCA.CertPool,
		MinVersion:   tls.VersionTLS12,
	}

	mtlsSrv := &http.Server{
		Addr:         mtlsAddr,
		Handler:      server.AgentRoutes(),
		TLSConfig:    tlsConfig,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
	}
	go func() {
		log.Printf("Listener mTLS agents démarré sur %s", mtlsAddr)
		if err := mtlsSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serveur mTLS: %v", err)
		}
	}()

	select {}
}

func staleAgentSweeper(repos *repository.Repositories) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if err := repos.Agents.MarkStaleOffline(context.Background(), 2*time.Minute); err != nil {
			log.Printf("balayage agents inactifs: %v", err)
		}
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
