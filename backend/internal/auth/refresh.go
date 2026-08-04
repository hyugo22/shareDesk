package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewRefreshToken génère un token opaque aléatoire (256 bits). Seul son hash
// SHA-256 est stocké en base — le token en clair n'est jamais persisté et
// n'est renvoyé qu'une seule fois au client au moment de l'émission.
func NewRefreshToken() (token string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
