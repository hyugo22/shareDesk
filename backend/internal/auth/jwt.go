package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID      string   `json:"uid"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Permissions []string `json:"perms"`
	jwt.RegisteredClaims
}

type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTIssuer(secret []byte, ttl time.Duration) *JWTIssuer {
	return &JWTIssuer{secret: secret, ttl: ttl}
}

func (j *JWTIssuer) Issue(userID, email, role string, permissions []string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:      userID,
		Email:       email,
		Role:        role,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
			Subject:   userID,
			Issuer:    "sharedesk-backend",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *JWTIssuer) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("algorithme de signature JWT inattendu")
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token invalide")
	}
	return claims, nil
}
