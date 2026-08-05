// Commande agent : agent edge ShareDesk (enrôlement mTLS, capture d'écran,
// injection clavier/souris, session de contrôle à distance WebRTC).
//
// Peut tourner au premier plan (usage manuel/dev, configuration via
// variables d'environnement) ou être installé comme service système
// (Windows/systemd/launchd via github.com/kardianos/service) avec
// `sharedesk-agent install --server-url=... --mtls-host=... --token=...` :
// dans ce cas la configuration est persistée sur disque (internal/svcconfig)
// et l'enrôlement est effectué une fois pour toutes au moment de l'install.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kardianos/service"
	"github.com/pion/webrtc/v4"

	"github.com/hyugo22/sharedesk/agent/internal/capture"
	"github.com/hyugo22/sharedesk/agent/internal/identity"
	"github.com/hyugo22/sharedesk/agent/internal/inject"
	"github.com/hyugo22/sharedesk/agent/internal/rtcsession"
	"github.com/hyugo22/sharedesk/agent/internal/statusapi"
	"github.com/hyugo22/sharedesk/agent/internal/svcconfig"
	"github.com/hyugo22/sharedesk/agent/internal/wsclient"
)

const agentVersion = "0.1.0-dev"

// trayExeName est le nom attendu de l'icône de zone de notification (voir
// agent/cmd/tray), installée à côté du binaire de l'agent par le MSI.
const trayExeName = "sharedesk-agent-tray.exe"

var svcSpec = &service.Config{
	Name:        "ShareDeskAgent",
	DisplayName: "ShareDesk Remote Support Agent",
	Description: "Agent de prise en main à distance ShareDesk : connexion mTLS au serveur, capture d'écran, contrôle clavier/souris à la demande.",
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			cmdInstall(os.Args[2:])
			return
		case "uninstall":
			cmdServiceControl("uninstall")
			return
		case "start":
			cmdServiceControl("start")
			return
		case "stop":
			cmdServiceControl("stop")
			return
		case "run":
			// traité ci-dessous comme le mode par défaut
		default:
			fmt.Fprintf(os.Stderr, "commande inconnue: %s\n\nUsage: sharedesk-agent [install|uninstall|start|stop|run]\n", os.Args[1])
			os.Exit(2)
		}
	}

	prg := &program{}
	svc, err := service.New(prg, svcSpec)
	if err != nil {
		log.Fatalf("initialisation service: %v", err)
	}
	prg.logger, _ = svc.Logger(nil)

	if err := svc.Run(); err != nil {
		log.Fatalf("exécution: %v", err)
	}
}

// --- Sous-commande "install" : enrôle l'agent puis l'enregistre comme service ---

func cmdInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	serverURL := fs.String("server-url", "", "URL du backend, ex: https://sharedesk.example.com:8080")
	mtlsHost := fs.String("mtls-host", "", "hôte:port du listener mTLS agents, ex: sharedesk.example.com:8443")
	token := fs.String("token", "", "token d'enrôlement à usage unique généré depuis l'interface")
	dataDir := fs.String("data-dir", "", "répertoire de données (défaut: sous-dossier du répertoire de configuration)")
	_ = fs.Parse(args)

	if *serverURL == "" || *mtlsHost == "" || *token == "" {
		fmt.Fprintln(os.Stderr, "usage: sharedesk-agent install --server-url=... --mtls-host=... --token=...")
		os.Exit(2)
	}

	configDir := svcconfig.DefaultDir()
	dd := *dataDir
	if dd == "" {
		dd = filepath.Join(configDir, "data")
	}

	if !identity.IsEnrolled(dd) {
		hostname, _ := os.Hostname()
		fmt.Println("Enrôlement auprès du serveur…")
		if _, err := identity.Enroll(*serverURL, *token, hostname, runtime.GOOS, "", runtime.GOARCH, agentVersion, dd); err != nil {
			log.Fatalf("enrôlement échoué: %v", err)
		}
		fmt.Println("Agent enrôlé avec succès.")
	} else {
		fmt.Println("Agent déjà enrôlé, configuration mise à jour.")
	}

	if err := svcconfig.Save(configDir, svcconfig.Config{ServerURL: *serverURL, MTLSHost: *mtlsHost, DataDir: dd}); err != nil {
		log.Fatalf("écriture de la configuration: %v", err)
	}

	prg := &program{}
	svc, err := service.New(prg, svcSpec)
	if err != nil {
		log.Fatalf("initialisation service: %v", err)
	}
	if err := svc.Install(); err != nil {
		log.Fatalf("installation du service: %v", err)
	}
	if err := svc.Start(); err != nil {
		log.Fatalf("démarrage du service: %v", err)
	}
	fmt.Println("Service installé et démarré.")

	if exePath, err := os.Executable(); err == nil {
		trayPath := filepath.Join(filepath.Dir(exePath), trayExeName)
		if _, statErr := os.Stat(trayPath); statErr == nil {
			if err := registerTrayAutostart(trayPath); err != nil {
				fmt.Fprintf(os.Stderr, "avertissement: icône de zone de notification non enregistrée au démarrage: %v\n", err)
			} else {
				fmt.Println("Icône de statut enregistrée (visible à la prochaine connexion).")
			}
		}
	}
}

func cmdServiceControl(action string) {
	prg := &program{}
	svc, err := service.New(prg, svcSpec)
	if err != nil {
		log.Fatalf("initialisation service: %v", err)
	}
	if err := service.Control(svc, action); err != nil {
		log.Fatalf("%s: %v", action, err)
	}
	fmt.Printf("%s effectué.\n", action)
}

// --- Program : cycle de vie géré par le gestionnaire de services ---

type program struct {
	cancel context.CancelFunc
	logger service.Logger
	status *statusapi.Store
}

func (p *program) Start(s service.Service) error {
	p.status = statusapi.NewStore()
	go func() {
		if err := p.status.ListenAndServe(); err != nil {
			p.logf("API de statut locale indisponible (port %d déjà utilisé ?): %v", statusapi.DefaultPort, err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.run(ctx)
	return nil
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

// run résout la configuration (fichier persistant si l'agent a été installé
// comme service, sinon variables d'environnement pour l'usage manuel/dev) et
// boucle indéfiniment sur la connexion au serveur jusqu'à annulation de ctx.
func (p *program) run(ctx context.Context) {
	cfg, err := svcconfig.Load(svcconfig.DefaultDir())
	dataDir := getEnv("AGENT_DATA_DIR", "./sharedesk-agent-data")
	serverURL := os.Getenv("SERVER_URL")
	mtlsHost := os.Getenv("SERVER_MTLS_HOST")
	if err == nil {
		dataDir, serverURL, mtlsHost = cfg.DataDir, cfg.ServerURL, cfg.MTLSHost
	}

	id, enrollErr := ensureEnrolled(dataDir, serverURL)
	if enrollErr != nil {
		p.fatal(fmt.Errorf("enrôlement: %w", enrollErr))
		return
	}
	p.logf("agent enrôlé (id=%s)", id.AgentID)
	p.status.Update(func(st *statusapi.Status) {
		st.AgentID, st.Server = id.AgentID, serverURL
	})

	ice := iceServers()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := runSession(ctx, id, mtlsHost, ice, p); err != nil {
			p.logf("connexion au serveur perdue: %v — nouvelle tentative dans 10s", err)
			p.status.Update(func(st *statusapi.Status) {
				st.Connected, st.SessionActive, st.LastError = false, false, err.Error()
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func (p *program) logf(format string, args ...any) {
	log.Printf(format, args...)
	if p.logger != nil {
		_ = p.logger.Info(fmt.Sprintf(format, args...))
	}
}

// fatal signale une erreur de démarrage irrécupérable. En exécution
// interactive (double-clic, terminal manuel — pas un service installé), la
// fenêtre de console se refermerait sinon instantanément sur un simple
// os.Exit : on affiche donc un message explicite et on attend une touche.
func (p *program) fatal(err error) {
	log.Printf("erreur fatale au démarrage: %v", err)
	if p.logger != nil {
		_ = p.logger.Error(err.Error())
	}
	if p.status != nil {
		p.status.Update(func(st *statusapi.Status) { st.LastError = err.Error() })
	}
	if service.Interactive() {
		fmt.Fprintln(os.Stderr, "\nL'agent n'a pas pu démarrer :", err)
		fmt.Fprintln(os.Stderr, "\nCet exécutable ne fonctionne pas en double-clic direct : il doit être")
		fmt.Fprintln(os.Stderr, "installé avec les paramètres d'enrôlement, depuis une invite de commandes :")
		fmt.Fprintln(os.Stderr, `  sharedesk-agent.exe install --server-url="https://VOTRE_SERVEUR:8080" --mtls-host="VOTRE_SERVEUR:8443" --token="TOKEN_A_USAGE_UNIQUE"`)
		fmt.Fprintln(os.Stderr, "\nAppuie sur Entrée pour fermer…")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(1)
}

func ensureEnrolled(dataDir, serverURL string) (*identity.Identity, error) {
	if identity.IsEnrolled(dataDir) {
		return identity.Load(dataDir)
	}
	token, err := mustGetEnv("ENROLLMENT_TOKEN")
	if err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	return identity.Enroll(serverURL, token, hostname, runtime.GOOS, "", runtime.GOARCH, agentVersion, dataDir)
}

func runSession(ctx context.Context, id *identity.Identity, mtlsHost string, ice []webrtc.ICEServer, p *program) error {
	tlsConfig := id.TLSConfig(hostOnly(mtlsHost))
	client, err := wsclient.Dial(mtlsHost, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()
	p.logf("connecté au serveur (%s)", mtlsHost)
	p.status.Update(func(st *statusapi.Status) { st.Connected, st.LastError = true, "" })
	defer p.status.Update(func(st *statusapi.Status) { st.Connected, st.SessionActive = false, false })

	cap := capture.NewProvider()
	inj := inject.NewProvider()

	var current *rtcsession.Session
	closeCurrent := func() {
		if current != nil {
			current.Close()
			current = nil
		}
		p.status.Update(func(st *statusapi.Status) { st.SessionActive = false })
	}
	defer closeCurrent()

	go func() {
		<-ctx.Done()
		client.Close()
	}()

	for msg := range client.Message {
		if err := handleSignal(msg, &current, cap, inj, ice, client, p.status); err != nil {
			p.logf("%v", err)
		}
	}

	return nil
}

func handleSignal(msg wsclient.Message, current **rtcsession.Session, cap capture.Provider, inj inject.Provider, ice []webrtc.ICEServer, client *wsclient.Client, status *statusapi.Store) error {
	switch msg.Type {
	case "offer":
		if *current != nil {
			(*current).Close()
			*current = nil
		}
		var payload struct {
			SDP string `json:"sdp"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return nil
		}
		sess, err := rtcsession.NewSession(cap, inj, ice)
		if err != nil {
			return fmt.Errorf("création session WebRTC: %w", err)
		}
		sessionID := msg.SessionID
		sess.OnICECandidate(func(c webrtc.ICECandidateInit) {
			payload, _ := json.Marshal(c)
			_ = client.Send(wsclient.Message{Type: "ice-candidate", SessionID: sessionID, Payload: payload})
		})
		answerSDP, err := sess.HandleOffer(payload.SDP)
		if err != nil {
			sess.Close()
			return fmt.Errorf("traitement offer: %w", err)
		}
		*current = sess
		status.Update(func(st *statusapi.Status) { st.SessionActive = true })
		answerPayload, _ := json.Marshal(map[string]string{"sdp": answerSDP})
		return client.Send(wsclient.Message{Type: "answer", SessionID: sessionID, Payload: answerPayload})

	case "ice-candidate":
		if *current != nil {
			return (*current).HandleICECandidate(msg.Payload)
		}

	case "session-end":
		if *current != nil {
			(*current).Close()
			*current = nil
		}
		status.Update(func(st *statusapi.Status) { st.SessionActive = false })
	}
	return nil
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

func mustGetEnv(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("variable d'environnement requise manquante: %s", key)
	}
	return v, nil
}
