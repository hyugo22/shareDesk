// Package rtcsession établit la connexion WebRTC agent<->navigateur pour une
// session de contrôle. Le canal clavier/souris ("control") est créé par le
// navigateur ; l'agent y répond et ouvre son propre canal ("frames") pour
// l'envoi des images d'écran.
//
// Limitation connue v1 : faute d'encodeur vidéo natif (VP8/H.264) intégré,
// les images sont capturées à basse fréquence, encodées en JPEG et envoyées
// telles quelles sur le DataChannel "frames" — ce n'est pas un flux vidéo
// WebRTC à proprement parler (pas de piste média), mais un relais d'images
// fonctionnel de bout en bout sur un canal WebRTC réel et chiffré (DTLS).
// Le prochain palier consiste à brancher un encodeur VP8/H.264 (ex. bindings
// libvpx) sur une vraie piste vidéo WebRTC pour un flux temps réel fluide.
package rtcsession

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"log"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/hyugo22/sharedesk/agent/internal/capture"
	"github.com/hyugo22/sharedesk/agent/internal/inject"
)

const frameInterval = 150 * time.Millisecond // ~6-7 fps

type inputEvent struct {
	Type   string `json:"type"` // mousemove | mousedown | mouseup | wheel | keydown | keyup
	X      int32  `json:"x,omitempty"`
	Y      int32  `json:"y,omitempty"`
	Button string `json:"button,omitempty"`
	DX     int32  `json:"dx,omitempty"`
	DY     int32  `json:"dy,omitempty"`
	Key    uint16 `json:"key,omitempty"`
}

type Session struct {
	pc      *webrtc.PeerConnection
	capture capture.Provider
	inject  inject.Provider
	closed  atomic.Bool
}

func NewSession(cap capture.Provider, inj inject.Provider, ice []webrtc.ICEServer) (*Session, error) {
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: ice})
	if err != nil {
		return nil, err
	}
	s := &Session{pc: pc, capture: cap, inject: inj}

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() != "control" {
			return
		}
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			s.handleInput(msg.Data)
		})
	})

	framesDC, err := pc.CreateDataChannel("frames", nil)
	if err != nil {
		pc.Close()
		return nil, err
	}
	framesDC.OnOpen(func() {
		go s.frameLoop(framesDC)
	})

	return s, nil
}

func (s *Session) handleInput(raw []byte) {
	var ev inputEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return
	}
	var err error
	switch ev.Type {
	case "mousemove":
		err = s.inject.MoveMouse(ev.X, ev.Y)
	case "mousedown":
		err = s.inject.MouseButtonEvent(buttonFromString(ev.Button), true)
	case "mouseup":
		err = s.inject.MouseButtonEvent(buttonFromString(ev.Button), false)
	case "wheel":
		err = s.inject.MouseWheel(ev.DX, ev.DY)
	case "keydown":
		err = s.inject.KeyEvent(ev.Key, true)
	case "keyup":
		err = s.inject.KeyEvent(ev.Key, false)
	}
	if err != nil {
		log.Printf("rtcsession: injection échouée (%s): %v", ev.Type, err)
	}
}

func buttonFromString(s string) inject.MouseButton {
	switch s {
	case "right":
		return inject.MouseRight
	case "middle":
		return inject.MouseMiddle
	default:
		return inject.MouseLeft
	}
}

func (s *Session) frameLoop(dc *webrtc.DataChannel) {
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()
	for range ticker.C {
		if s.closed.Load() {
			return
		}
		img, err := s.capture.CaptureFrame()
		if err != nil {
			continue
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 55}); err != nil {
			continue
		}
		if err := dc.Send(buf.Bytes()); err != nil {
			return
		}
	}
}

func (s *Session) HandleOffer(sdp string) (answerSDP string, err error) {
	if err := s.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}); err != nil {
		return "", err
	}
	answer, err := s.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}
	if err := s.pc.SetLocalDescription(answer); err != nil {
		return "", err
	}
	return answer.SDP, nil
}

func (s *Session) HandleICECandidate(candidateJSON []byte) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(candidateJSON, &candidate); err != nil {
		return fmt.Errorf("candidat ICE invalide: %w", err)
	}
	return s.pc.AddICECandidate(candidate)
}

func (s *Session) OnICECandidate(fn func(candidate webrtc.ICECandidateInit)) {
	s.pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		fn(c.ToJSON())
	})
}

func (s *Session) Close() error {
	s.closed.Store(true)
	return s.pc.Close()
}
