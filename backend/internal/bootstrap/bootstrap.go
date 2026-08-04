// Package bootstrap crée le premier compte administrateur au tout premier
// démarrage (base vide), pour permettre la connexion initiale à l'interface.
package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/hyugo22/sharedesk/backend/internal/auth"
	"github.com/hyugo22/sharedesk/backend/internal/models"
	"github.com/hyugo22/sharedesk/backend/internal/repository"
)

func EnsureInitialAdmin(ctx context.Context, repos *repository.Repositories) error {
	count, err := repos.Users.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
	if email == "" {
		email = "admin@sharedesk.local"
	}
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	generated := password == ""
	if generated {
		raw := make([]byte, 18)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		password = base64.RawURLEncoding.EncodeToString(raw)
	}

	adminRole, err := repos.Roles.GetByName(ctx, "admin")
	if err != nil {
		return fmt.Errorf("rôle admin introuvable (migrations non appliquées ?): %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	user := &models.User{Email: email, DisplayName: "Administrateur", PasswordHash: hash, RoleID: adminRole.ID}
	if err := repos.Users.Create(ctx, user); err != nil {
		return err
	}

	if generated {
		log.Printf("=================================================================")
		log.Printf(" Premier démarrage : compte administrateur créé")
		log.Printf(" Email    : %s", email)
		log.Printf(" Mot de passe (généré, à changer immédiatement) : %s", password)
		log.Printf("=================================================================")
	} else {
		log.Printf("Premier démarrage : compte administrateur créé (%s)", email)
	}
	return nil
}
