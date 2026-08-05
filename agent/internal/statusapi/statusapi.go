// Package statusapi expose l'état courant de l'agent (connecté au serveur,
// session de contrôle en cours…) sur une petite API HTTP locale, uniquement
// accessible en boucle locale (127.0.0.1). Un service Windows ne peut pas
// afficher d'interface sur le bureau de l'utilisateur connecté (isolation de
// session) : l'icône de zone de notification (cmd/tray) est un programme
// séparé qui interroge cette API pour savoir quoi afficher.
package statusapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// DefaultPort n'est volontairement pas configurable pour l'instant : agent
// et tray doivent s'accorder sur la même valeur sans mécanisme de découverte.
const DefaultPort = 47812

type Status struct {
	Connected     bool      `json:"connected"`
	Server        string    `json:"server,omitempty"`
	AgentID       string    `json:"agent_id,omitempty"`
	SessionActive bool      `json:"session_active"`
	LastError     string    `json:"last_error,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Store struct {
	mu     sync.RWMutex
	status Status
}

func NewStore() *Store { return &Store{} }

func (s *Store) Update(fn func(*Status)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.status)
	s.status.UpdatedAt = time.Now()
}

func (s *Store) Get() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.Get())
}

// ListenAndServe démarre l'API de statut, strictement en boucle locale : ni
// le réseau de l'agent ni le serveur central n'y ont accès, seuls les
// processus tournant sur la même machine (typiquement l'icône de zone de
// notification, cmd/tray) le peuvent.
func (s *Store) ListenAndServe() error {
	addr := "127.0.0.1:" + strconv.Itoa(DefaultPort)
	return http.ListenAndServe(addr, s)
}
