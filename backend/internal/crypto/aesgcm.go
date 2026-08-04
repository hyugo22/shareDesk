// Package crypto fournit le chiffrement applicatif (AES-256-GCM) des champs
// sensibles stockés en base (tokens d'agents, secrets d'intégration, etc.).
// La clé est fournie par la configuration (variable d'environnement / secret
// externe) et n'est jamais codée en dur.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const KeySize = 32 // AES-256

type Sealer struct {
	gcm cipher.AEAD
}

func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("clé de chiffrement invalide: attendu %d octets, reçu %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{gcm: gcm}, nil
}

// Seal chiffre plaintext et retourne nonce||ciphertext encodé en base64.
func (s *Sealer) Seal(plaintext []byte) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := s.gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *Sealer) SealString(plaintext string) (string, error) {
	return s.Seal([]byte(plaintext))
}

// Open déchiffre une valeur produite par Seal.
func (s *Sealer) Open(encoded string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	nonceSize := s.gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, errors.New("valeur chiffrée corrompue: trop courte")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	return s.gcm.Open(nil, nonce, ciphertext, nil)
}

func (s *Sealer) OpenString(encoded string) (string, error) {
	pt, err := s.Open(encoded)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// DecodeBase64Key décode une clé AES-256 encodée en base64 (ex: `openssl rand -base64 32`).
func DecodeBase64Key(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("la clé doit faire %d octets une fois décodée, reçu %d", KeySize, len(key))
	}
	return key, nil
}
