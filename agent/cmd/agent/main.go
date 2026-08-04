// Commande agent : agent edge ShareDesk (enrôlement mTLS, capture d'écran,
// injection clavier/souris, session de contrôle à distance WebRTC).
package main

import (
	"encoding/json"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/hyugo22/sharedesk/agent/internal/capture"
	"github.com/hyugo22/sharedesk/agent/internal/identity"
	"github.com/hyugo22/sharedesk/agent/internal/inject"
	"github.com/hyugo22/sharedesk/agent/internal/rtcsession"
	"github.com/hyugo22/sharedesk/agent/internal/wsclient"
)

const agentVersion = "0.1.0-dev"

func main() {
	dataDir := getEnv("AGENT_DATA_DIR", "./sharedesk-agent-data")
	serverURL := mustGetEnv("SERVER_URL")   // ex: https://sharedesk.example.com
	mtlsHost := mustGetEnv("SERVER_MTLS_HOST") // ex: sharedesk.example.com:8443

	id, err := ensureEnrolled(dataDir, serverURL)
	if err != nil {
		log.Fatalf("enrôlement: %v", err)
	}
	log.Printf("agent enrôlé (id=%s)", id.AgentID)

	ice := iceServers()

	for {
		if err := runSession(id, mtlsHost, ice); err != nil {
			log.Printf("connexion au serveur perdue: %v — nouvelle tentative dans 10s", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func ensureEnrolled(dataDir, serverURL string) (*identity.Identity, error) {
	if identity.IsEnrolled(dataDir) {
		return identity.Load(dataDir)
	}

	token := mustGetEnv("ENROLLMENT_TOKEN")
	hostname, _ := os.Hostname()
	return identity.Enroll(serverURL, token, hostname, runtime.GOOS, "", runtime.GOARCH, agentVersion, dataDir)
}

func runSession(id *identity.Identity, mtlsHost string, ice []webrtc.ICEServer) error {
	tlsConfig := id.TLSConfig(hostOnly(mtlsHost))
	client, err := wsclient.Dial(mtlsHost, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()
	log.Printf("connecté au serveur (%s)", mtlsHost)

	cap := capture.NewProvider()
	inj := inject.NewProvider()

	var current *rtcsession.Session
	closeCurrent := func() {
		if current != nil {
			current.Close()
			current = nil
		}
	}
	defer closeCurrent()

	for msg := range client.Message {
		switch msg.Type {
		case "offer":
			closeCurrent()
			var payload struct {
				SDP string `json:"sdp"`
			}
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}
			sess, err := rtcsession.NewSession(cap, inj, ice)
			if err != nil {
				log.Printf("création session WebRTC: %v", err)
				continue
			}
			sessionID := msg.SessionID
			sess.OnICECandidate(func(c webrtc.ICECandidateInit) {
				payload, _ := json.Marshal(c)
				_ = client.Send(wsclient.Message{Type: "ice-candidate", SessionID: sessionID, Payload: payload})
			})
			answerSDP, err := sess.HandleOffer(payload.SDP)
			if err != nil {
				log.Printf("traitement offer: %v", err)
				sess.Close()
				continue
			}
			current = sess
			answerPayload, _ := json.Marshal(map[string]string{"sdp": answerSDP})
			_ = client.Send(wsclient.Message{Type: "answer", SessionID: sessionID, Payload: answerPayload})

		case "ice-candidate":
			if current != nil {
				_ = current.HandleICECandidate(msg.Payload)
			}

		case "session-end":
			closeCurrent()
		}
	}

	return nil // connexion fermée proprement par le pair
}

func iceServers() []webrtc.ICEServer {
	turnURL := os.Getenv("TURN_URL")
	if turnURL == "" {
		return nil
	}
	return []webrtc.ICEServer{{
		URLs:       []string{turnURL},
		Username:   os.Getenv("TURN_USERNAME"),
		Credential: os.Getenv("TURN_CREDENTIAL"),
	}}
}

func hostOnly(hostPort string) string {
	for i := len(hostPort) - 1; i >= 0; i-- {
		if hostPort[i] == ':' {
			return hostPort[:i]
		}
	}
	return hostPort
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		log.Fatalf("variable d'environnement requise manquante: %s", key)
	}
	return v
}
