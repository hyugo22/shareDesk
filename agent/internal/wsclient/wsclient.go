// Package wsclient maintient la connexion WebSocket mTLS de l'agent vers le
// backend (signalisation WebRTC + statut). L'authentification est entièrement
// portée par le certificat client présenté lors de la poignée de main TLS.
package wsclient

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type Client struct {
	conn    *websocket.Conn
	Message chan Message
}

func Dial(host string, tlsConfig *tls.Config) (*Client, error) {
	u := url.URL{Scheme: "wss", Host: host, Path: "/agent/channel"}
	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: 15 * time.Second,
	}
	conn, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("connexion refusée par le serveur (statut %d): %w", resp.StatusCode, err)
		}
		return nil, err
	}

	c := &Client{conn: conn, Message: make(chan Message, 16)}
	go c.readLoop()
	return c, nil
}

func (c *Client) readLoop() {
	defer close(c.Message)
	for {
		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			log.Printf("wsclient: connexion fermée: %v", err)
			return
		}
		c.Message <- msg
	}
}

func (c *Client) Send(msg Message) error {
	return c.conn.WriteJSON(msg)
}

func (c *Client) Close() error {
	return c.conn.Close()
}
