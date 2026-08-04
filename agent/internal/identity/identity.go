// Package identity gère l'identité cryptographique de l'agent : génération
// locale de la paire de clés (la clé privée ne quitte jamais la machine),
// enrôlement auprès du backend (échange du token à usage unique contre un
// certificat client mTLS), et épinglage du certificat de la CA serveur.
//
// Une fois enrôlé, l'agent ne fait plus jamais confiance à un autre émetteur
// que celui épinglé lors de l'enrôlement : voir Transport() dans le package
// internal/transport, qui construit un tls.Config n'utilisant que ce pin,
// jamais le magasin de certificats système.
package identity

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const (
	keyFile    = "agent.key"
	certFile   = "agent.crt"
	caCertFile = "ca.crt"
	idFile     = "agent.id"
)

type Identity struct {
	AgentID string
	dataDir string
	cert    tls.Certificate
	caPool  *x509.CertPool
}

func IsEnrolled(dataDir string) bool {
	return fileExists(filepath.Join(dataDir, keyFile)) &&
		fileExists(filepath.Join(dataDir, certFile)) &&
		fileExists(filepath.Join(dataDir, caCertFile))
}

// Load charge l'identité déjà enrôlée depuis dataDir.
func Load(dataDir string) (*Identity, error) {
	keyPEM, err := os.ReadFile(filepath.Join(dataDir, keyFile))
	if err != nil {
		return nil, err
	}
	certPEM, err := os.ReadFile(filepath.Join(dataDir, certFile))
	if err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(filepath.Join(dataDir, caCertFile))
	if err != nil {
		return nil, err
	}
	agentID, err := os.ReadFile(filepath.Join(dataDir, idFile))
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("certificat/clé agent invalides: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("certificat CA épinglé illisible")
	}

	return &Identity{AgentID: string(agentID), dataDir: dataDir, cert: cert, caPool: pool}, nil
}

// Enroll échange un token d'enrôlement à usage unique contre un certificat
// client mTLS. La clé privée est générée localement et n'est jamais transmise.
func Enroll(serverBaseURL, enrollmentToken, hostname, osName, osVersion, arch, agentVersion, dataDir string) (*Identity, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(filepath.Join(dataDir, keyFile), keyPEM, 0o600); err != nil {
		return nil, err
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: hostname},
	}, priv)
	if err != nil {
		return nil, err
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	reqBody, err := json.Marshal(map[string]string{
		"enrollment_token": enrollmentToken,
		"hostname":         hostname,
		"os":               osName,
		"os_version":       osVersion,
		"arch":             arch,
		"agent_version":    agentVersion,
		"csr_pem":          string(csrPEM),
	})
	if err != nil {
		return nil, err
	}

	// Bootstrap uniquement : cette requête utilise le TLS serveur classique
	// (magasin de confiance système), car l'agent n'a pas encore de CA à
	// épingler. La sécurité repose ici sur le token à usage unique, obtenu
	// hors-bande par l'administrateur via l'interface authentifiée.
	resp, err := http.Post(serverBaseURL+"/api/v1/agents/enroll", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("requête d'enrôlement: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("enrôlement refusé par le serveur (statut %d)", resp.StatusCode)
	}

	var out struct {
		AgentID   string `json:"agent_id"`
		CertPEM   string `json:"cert_pem"`
		CACertPEM string `json:"ca_cert_pem"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(dataDir, certFile), []byte(out.CertPEM), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dataDir, caCertFile), []byte(out.CACertPEM), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dataDir, idFile), []byte(out.AgentID), 0o600); err != nil {
		return nil, err
	}

	return Load(dataDir)
}

// TLSConfig construit la configuration mTLS épinglée : seul le certificat de
// la CA reçu lors de l'enrôlement est reconnu, jamais le magasin système.
func (id *Identity) TLSConfig(serverName string) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{id.cert},
		RootCAs:      id.caPool,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
