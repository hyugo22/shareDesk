package httpapi

import (
	"net/http"

	"github.com/hyugo22/sharedesk/backend/internal/directory"
)

func (s *Server) handleGetLDAPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.repos.LDAPConfig.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	// BindPasswordEnc n'est jamais sérialisé (json:"-") : seul HasBindPassword l'indique.
	writeJSON(w, http.StatusOK, cfg)
}

type setLDAPConfigRequest struct {
	Enabled              bool              `json:"enabled"`
	Host                 string            `json:"host"`
	Port                 int               `json:"port"`
	ConnectionMode       string            `json:"connection_mode"`
	BindDN               string            `json:"bind_dn"`
	BindPassword         *string           `json:"bind_password"` // omis = conserver l'existant
	BaseDN               string            `json:"base_dn"`
	AttributeMapping     map[string]any    `json:"attribute_mapping"`
	GroupRoleMapping     map[string]any    `json:"group_role_mapping"`
	SyncIntervalMinutes  int               `json:"sync_interval_minutes"`
}

func (s *Server) handleSetLDAPConfig(w http.ResponseWriter, r *http.Request) {
	var req setLDAPConfigRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return
	}
	if req.ConnectionMode != "ldaps" && req.ConnectionMode != "starttls" {
		writeError(w, http.StatusBadRequest, "connection_mode doit être 'ldaps' ou 'starttls' (LDAP en clair interdit)")
		return
	}

	cfg, err := s.repos.LDAPConfig.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	cfg.Enabled, cfg.Host, cfg.Port = req.Enabled, req.Host, req.Port
	cfg.ConnectionMode, cfg.BindDN, cfg.BaseDN = req.ConnectionMode, req.BindDN, req.BaseDN
	cfg.AttributeMapping, cfg.GroupRoleMapping = req.AttributeMapping, req.GroupRoleMapping
	cfg.SyncIntervalMinutes = req.SyncIntervalMinutes

	var encPassword *string
	if req.BindPassword != nil && *req.BindPassword != "" {
		enc, err := s.sealer.SealString(*req.BindPassword)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		encPassword = &enc
	}

	claims := claimsFromContext(r.Context())
	if err := s.repos.LDAPConfig.Upsert(r.Context(), cfg, encPassword, claims.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// Le mot de passe n'est jamais inclus dans le détail journalisé.
	s.audit(r, "user", claims.UserID, "settings.ldap.update", "ldap_config", "", map[string]any{
		"enabled": req.Enabled, "host": req.Host, "connection_mode": req.ConnectionMode,
		"password_changed": encPassword != nil,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestLDAPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.repos.LDAPConfig.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	var password string
	if cfg.HasBindPassword {
		password, _ = s.sealer.OpenString(cfg.BindPasswordEnc)
	}

	dCfg := directory.Config{
		Host: cfg.Host, Port: cfg.Port, ConnectionMode: cfg.ConnectionMode,
		BindDN: cfg.BindDN, BindPassword: password, BaseDN: cfg.BaseDN,
	}
	claims := claimsFromContext(r.Context())
	if testErr := s.directory.TestConnection(r.Context(), dCfg); testErr != nil {
		s.audit(r, "user", claims.UserID, "settings.ldap.test", "ldap_config", "", map[string]any{"success": false})
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "error": testErr.Error()})
		return
	}
	s.audit(r, "user", claims.UserID, "settings.ldap.test", "ldap_config", "", map[string]any{"success": true})
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleSyncLDAP(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.repos.LDAPConfig.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	var password string
	if cfg.HasBindPassword {
		password, _ = s.sealer.OpenString(cfg.BindPasswordEnc)
	}

	dCfg := directory.Config{
		Host: cfg.Host, Port: cfg.Port, ConnectionMode: cfg.ConnectionMode,
		BindDN: cfg.BindDN, BindPassword: password, BaseDN: cfg.BaseDN,
	}
	claims := claimsFromContext(r.Context())
	result, syncErr := s.directory.SyncUsers(r.Context(), dCfg)
	status := "success"
	if syncErr != nil {
		status = "error: " + syncErr.Error()
	}
	_ = s.repos.LDAPConfig.RecordSyncResult(r.Context(), status)
	s.audit(r, "user", claims.UserID, "settings.ldap.sync", "ldap_config", "", map[string]any{"status": status})

	if syncErr != nil {
		writeError(w, http.StatusNotImplemented, syncErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
