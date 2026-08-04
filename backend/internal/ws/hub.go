// Package ws implémente le hub de signalisation temps réel : statut des
// agents (online/offline) et relais des messages WebRTC (offer/answer/ICE)
// entre un viewer (navigateur, authentifié JWT) et l'agent ciblé (authentifié
// mTLS). Le flux média lui-même (vidéo/audio/clavier-souris) ne transite
// jamais par le backend : seule la signalisation y passe.
package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type      string          `json:"type"` // "offer" | "answer" | "ice-candidate" | "session-end" | "status"
	SessionID string          `json:"session_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type agentConn struct {
	agentID string
	conn    *websocket.Conn
	mu      sync.Mutex // sérialise les écritures concurrentes sur la connexion
}

type viewerConn struct {
	sessionID string
	agentID   string
	conn      *websocket.Conn
	mu        sync.Mutex
}

type Hub struct {
	mu      sync.RWMutex
	agents  map[string]*agentConn    // agentID -> connexion
	viewers map[string]*viewerConn   // sessionID -> connexion viewer

	onAgentStatus func(agentID string, online bool)
}

func NewHub(onAgentStatus func(agentID string, online bool)) *Hub {
	return &Hub{
		agents:        make(map[string]*agentConn),
		viewers:       make(map[string]*viewerConn),
		onAgentStatus: onAgentStatus,
	}
}

// RegisterAgent enregistre la connexion d'un agent authentifié mTLS et
// consomme ses messages jusqu'à déconnexion.
func (h *Hub) RegisterAgent(agentID string, conn *websocket.Conn) {
	ac := &agentConn{agentID: agentID, conn: conn}

	h.mu.Lock()
	if existing, ok := h.agents[agentID]; ok {
		existing.conn.Close() // une seule connexion active par agent
	}
	h.agents[agentID] = ac
	h.mu.Unlock()

	if h.onAgentStatus != nil {
		h.onAgentStatus(agentID, true)
	}

	defer func() {
		h.mu.Lock()
		if h.agents[agentID] == ac {
			delete(h.agents, agentID)
		}
		h.mu.Unlock()
		if h.onAgentStatus != nil {
			h.onAgentStatus(agentID, false)
		}
		conn.Close()
	}()

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		h.routeFromAgent(agentID, msg)
	}
}

// RegisterViewer enregistre la connexion d'un navigateur pour une session de
// contrôle donnée et relaie ses messages vers l'agent ciblé.
func (h *Hub) RegisterViewer(sessionID, agentID string, conn *websocket.Conn) {
	vc := &viewerConn{sessionID: sessionID, agentID: agentID, conn: conn}

	h.mu.Lock()
	h.viewers[sessionID] = vc
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		if h.viewers[sessionID] == vc {
			delete(h.viewers, sessionID)
		}
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		msg.SessionID = sessionID
		h.routeToAgent(agentID, msg)
	}
}

func (h *Hub) routeFromAgent(agentID string, msg Message) {
	h.mu.RLock()
	vc, ok := h.viewers[msg.SessionID]
	h.mu.RUnlock()
	if !ok || vc.agentID != agentID {
		return
	}
	vc.mu.Lock()
	defer vc.mu.Unlock()
	if err := vc.conn.WriteJSON(msg); err != nil {
		log.Printf("ws: échec relais agent->viewer (session=%s): %v", msg.SessionID, err)
	}
}

func (h *Hub) routeToAgent(agentID string, msg Message) {
	h.mu.RLock()
	ac, ok := h.agents[agentID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if err := ac.conn.WriteJSON(msg); err != nil {
		log.Printf("ws: échec relais viewer->agent (agent=%s): %v", agentID, err)
	}
}

// DisconnectAgent coupe immédiatement la connexion d'un agent (ex: révocation).
func (h *Hub) DisconnectAgent(agentID string) {
	h.mu.RLock()
	ac, ok := h.agents[agentID]
	h.mu.RUnlock()
	if ok {
		ac.conn.Close()
	}
}

func (h *Hub) IsAgentConnected(agentID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.agents[agentID]
	return ok
}
