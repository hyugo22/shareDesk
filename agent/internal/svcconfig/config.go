// Package svcconfig persiste la configuration de connexion de l'agent
// (adresse du serveur, répertoire de données) sur disque, pour qu'un agent
// installé en tant que service Windows/systemd/launchd puisse démarrer sans
// dépendre de variables d'environnement d'une session utilisateur.
package svcconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	ServerURL string `json:"server_url"`
	MTLSHost  string `json:"mtls_host"`
	DataDir   string `json:"data_dir"`
}

// DefaultDir retourne le répertoire de configuration/données par défaut,
// adapté à chaque plateforme (installation système, pas par utilisateur —
// le service tourne sous un compte système).
func DefaultDir() string {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "ShareDeskAgent")
	case "darwin":
		return "/Library/Application Support/ShareDeskAgent"
	default:
		return "/etc/sharedesk-agent"
	}
}

func configPath(dir string) string {
	return filepath.Join(dir, "config.json")
}

func Save(dir string, cfg Config) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(dir), raw, 0o600)
}

func Load(dir string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(configPath(dir))
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(raw, &cfg)
	return cfg, err
}
