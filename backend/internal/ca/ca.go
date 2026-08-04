// Package ca implémente une autorité de certification interne minimale,
// utilisée exclusivement pour l'authentification mutuelle (mTLS) entre le
// backend et les agents. Elle est indépendante de la CA publique éventuelle
// utilisée pour le TLS de l'API/frontend (hors périmètre de ce projet).
//
// Modèle de confiance :
//  1. Le certificat racine (CA) est généré au premier démarrage et persisté
//     chiffré sur disque (clé dérivée de CA_KEY_PASSPHRASE, jamais en clair).
//  2. Un certificat serveur, signé par cette CA, est utilisé par le listener
//     mTLS dédié aux agents.
//  3. Chaque agent reçoit un certificat client signé par cette CA lors de
//     l'enrôlement (voir internal/httpapi enrollment handlers) ; il épingle
//     le certificat de la CA à cette occasion et ne fait plus jamais confiance
//     à un autre émetteur ensuite.
//  4. Toute vérification de connexion agent passe par la CRL applicative
//     (table agent_cert_revocations) en plus de la validité du certificat.
package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/argon2"

	appcrypto "github.com/hyugo22/sharedesk/backend/internal/crypto"
)

const (
	caCertFile = "ca.crt"
	caKeyFile  = "ca.key.enc"
	saltFile   = "ca.salt"

	caValidity     = 10 * 365 * 24 * time.Hour
	serverValidity = 2 * 365 * 24 * time.Hour
	agentValidity  = 365 * 24 * time.Hour
)

type CA struct {
	cert     *x509.Certificate
	certPEM  []byte
	key      *ecdsa.PrivateKey
	CertPool *x509.CertPool
}

// LoadOrCreate charge la CA depuis dataDir, ou la génère si absente.
func LoadOrCreate(dataDir, passphrase string) (*CA, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}

	certPath := filepath.Join(dataDir, caCertFile)
	keyPath := filepath.Join(dataDir, caKeyFile)
	saltPath := filepath.Join(dataDir, saltFile)

	if fileExists(certPath) && fileExists(keyPath) && fileExists(saltPath) {
		return load(certPath, keyPath, saltPath, passphrase)
	}
	return create(dataDir, certPath, keyPath, saltPath, passphrase)
}

func create(dataDir, certPath, keyPath, saltPath, passphrase string) (*CA, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "ShareDesk Internal Agent CA", Organization: []string{"ShareDesk"}},
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return nil, err
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if err := os.WriteFile(saltPath, salt, 0o600); err != nil {
		return nil, err
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	sealer, err := appcrypto.NewSealer(derivePassphraseKey(passphrase, salt))
	if err != nil {
		return nil, err
	}
	sealedKey, err := sealer.SealString(string(keyPEM))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, []byte(sealedKey), 0o600); err != nil {
		return nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)

	return &CA{cert: cert, certPEM: certPEM, key: priv, CertPool: pool}, nil
}

func load(certPath, keyPath, saltPath, passphrase string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("ca.crt illisible")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	salt, err := os.ReadFile(saltPath)
	if err != nil {
		return nil, err
	}
	sealedKey, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	sealer, err := appcrypto.NewSealer(derivePassphraseKey(passphrase, salt))
	if err != nil {
		return nil, err
	}
	keyPEMStr, err := sealer.OpenString(string(sealedKey))
	if err != nil {
		return nil, fmt.Errorf("déchiffrement de la clé CA (mauvais CA_KEY_PASSPHRASE ?): %w", err)
	}
	keyBlock, _ := pem.Decode([]byte(keyPEMStr))
	if keyBlock == nil {
		return nil, errors.New("clé CA illisible après déchiffrement")
	}
	priv, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	pool.AddCert(cert)

	return &CA{cert: cert, certPEM: certPEM, key: priv, CertPool: pool}, nil
}

func (ca *CA) CertPEM() []byte { return ca.certPEM }

// IssueServerCert émet le certificat utilisé par le listener mTLS dédié aux agents.
func (ca *CA) IssueServerCert(hostnames []string) (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "sharedesk-backend"},
		NotBefore:    time.Now().Add(-5 * time.Minute),
		NotAfter:     time.Now().Add(serverValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     hostnames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &priv.PublicKey, ca.key)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}

// SignAgentCSR signe une CSR soumise par un agent lors de l'enrôlement et
// retourne le certificat client, son numéro de série (hex) et son empreinte SHA-256.
func (ca *CA) SignAgentCSR(csrPEM []byte, commonName string) (certPEM []byte, serialHex string, fingerprint string, err error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, "", "", errors.New("CSR PEM invalide")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, "", "", err
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, "", "", fmt.Errorf("signature CSR invalide: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, "", "", err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName, Organization: []string{"ShareDesk Agent"}},
		NotBefore:    time.Now().Add(-5 * time.Minute),
		NotAfter:     time.Now().Add(agentValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, csr.PublicKey, ca.key)
	if err != nil {
		return nil, "", "", err
	}

	sum := sha256.Sum256(der)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		serial.Text(16), hex.EncodeToString(sum[:]), nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}

func derivePassphraseKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, 3, 64*1024, 2, appcrypto.KeySize)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
