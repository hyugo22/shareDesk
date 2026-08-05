// Commande tray : icône de zone de notification affichant l'état de l'agent
// ShareDesk (connecté au serveur, session de contrôle en cours…). Programme
// séparé de l'agent lui-même : un service Windows tourne en Session 0,
// isolée du bureau de l'utilisateur connecté, et ne peut donc pas afficher
// d'interface — voir internal/statusapi pour l'API locale que ce programme
// interroge.
package main

import (
	"encoding/json"
	"image/color"
	"net/http"
	"strconv"
	"time"

	"github.com/getlantern/systray"

	"github.com/hyugo22/sharedesk/agent/internal/statusapi"
	"github.com/hyugo22/sharedesk/agent/internal/trayicon"
)

var (
	iconConnected      = trayicon.Circle(color.RGBA{34, 197, 94, 255})  // vert
	iconSessionActive  = trayicon.Circle(color.RGBA{59, 130, 246, 255}) // bleu
	iconDisconnected   = trayicon.Circle(color.RGBA{239, 68, 68, 255})  // rouge
	iconServiceMissing = trayicon.Circle(color.RGBA{148, 163, 184, 255}) // gris
)

func main() {
	systray.Run(onReady, func() {})
}

func onReady() {
	systray.SetIcon(iconServiceMissing)
	systray.SetTitle("")
	systray.SetTooltip("ShareDesk Agent — statut inconnu")

	statusItem := systray.AddMenuItem("Recherche du service…", "")
	statusItem.Disable()
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Fermer l'icône", "Ferme uniquement l'icône de statut — l'agent continue de tourner en service")

	go pollLoop(statusItem)

	go func() {
		<-quitItem.ClickedCh
		systray.Quit()
	}()
}

func pollLoop(statusItem *systray.MenuItem) {
	url := "http://127.0.0.1:" + strconv.Itoa(statusapi.DefaultPort) + "/status"
	client := &http.Client{Timeout: 3 * time.Second}

	for {
		st, err := fetchStatus(client, url)
		applyStatus(statusItem, st, err)
		time.Sleep(5 * time.Second)
	}
}

func fetchStatus(client *http.Client, url string) (*statusapi.Status, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var st statusapi.Status
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, err
	}
	return &st, nil
}

func applyStatus(item *systray.MenuItem, st *statusapi.Status, err error) {
	switch {
	case err != nil:
		systray.SetIcon(iconServiceMissing)
		systray.SetTooltip("ShareDesk Agent — service introuvable")
		item.SetTitle("Service introuvable ou arrêté")

	case st.SessionActive:
		systray.SetIcon(iconSessionActive)
		systray.SetTooltip("ShareDesk Agent — session de contrôle en cours")
		item.SetTitle("Session de contrôle en cours")

	case st.Connected:
		systray.SetIcon(iconConnected)
		systray.SetTooltip("ShareDesk Agent — connecté (" + st.Server + ")")
		item.SetTitle("Connecté à " + st.Server)

	default:
		systray.SetIcon(iconDisconnected)
		tooltip := "ShareDesk Agent — déconnecté"
		title := "Déconnecté du serveur"
		if st.LastError != "" {
			title = "Déconnecté : " + st.LastError
		}
		systray.SetTooltip(tooltip)
		item.SetTitle(title)
	}
}
