// Package config charge la configuration du backend depuis l'environnement.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hyugo22/sharedesk/backend/internal/crypto"
)

type Config struct {
	HTTPAddr  string
	PublicURL string

	// Origines autorisées pour les requêtes CORS du frontend (le navigateur
	// et l'API ne sont pas nécessairement sur la même origine, ex. dev local
	// avec des ports différents). Vide = aucune origine autorisée.
	CORSAllowedOrigins []string

	PostgresDSN string

	JWTSigningSecret []byte
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration

	AppEncryptionKey []byte // AES-256-GCM, 32 octets
	CAKeyPassphrase  string

	TURNRealm        string
	TURNSharedSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:         getEnv("BACKEND_HTTP_ADDR", ":8080"),
		PublicURL:        getEnv("BACKEND_PUBLIC_URL", "http://localhost:8080"),
		JWTSigningSecret: []byte(mustGetEnv("JWT_SIGNING_SECRET")),
		CAKeyPassphrase:  mustGetEnv("CA_KEY_PASSPHRASE"),
		TURNRealm:        getEnv("TURN_REALM", "sharedesk.local"),
		TURNSharedSecret: os.Getenv("TURN_SHARED_SECRET"),
	}
	cfg.CORSAllowedOrigins = splitAndTrim(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:8081"))

	var err error
	if cfg.AccessTokenTTL, err = time.ParseDuration(getEnv("ACCESS_TOKEN_TTL", "15m")); err != nil {
		return nil, fmt.Errorf("ACCESS_TOKEN_TTL invalide: %w", err)
	}
	if cfg.RefreshTokenTTL, err = time.ParseDuration(getEnv("REFRESH_TOKEN_TTL", "720h")); err != nil {
		return nil, fmt.Errorf("REFRESH_TOKEN_TTL invalide: %w", err)
	}

	key, err := crypto.DecodeBase64Key(mustGetEnv("APP_ENCRYPTION_KEY"))
	if err != nil {
		return nil, fmt.Errorf("APP_ENCRYPTION_KEY invalide: %w", err)
	}
	cfg.AppEncryptionKey = key

	cfg.PostgresDSN = buildPostgresDSN()

	if len(cfg.JWTSigningSecret) < 32 {
		return nil, fmt.Errorf("JWT_SIGNING_SECRET doit faire au moins 32 octets")
	}

	return cfg, nil
}

func buildPostgresDSN() string {
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	db := getEnv("POSTGRES_DB", "sharedesk")
	user := getEnv("POSTGRES_USER", "sharedesk_app")
	pass := os.Getenv("POSTGRES_PASSWORD")
	sslmode := getEnv("POSTGRES_SSLMODE", "require")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, db, sslmode)
}

func splitAndTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("variable d'environnement requise manquante: %s", key))
	}
	return v
}
