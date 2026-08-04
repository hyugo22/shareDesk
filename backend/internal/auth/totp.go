package auth

import (
	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret crée un nouveau secret TOTP pour l'enrôlement MFA d'un utilisateur.
// Le secret retourné doit être chiffré (internal/crypto) avant stockage en base.
func GenerateTOTPSecret(issuer, accountEmail string) (secret string, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountEmail,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

func VerifyTOTPCode(secret, code string) bool {
	return totp.Validate(code, secret)
}
