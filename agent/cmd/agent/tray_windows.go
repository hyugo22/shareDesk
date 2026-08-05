//go:build windows

package main

import "golang.org/x/sys/windows/registry"

// registerTrayAutostart inscrit l'icône de zone de notification au démarrage
// de session via la clé Run machine (HKLM, pas HKCU) : l'installation tourne
// en tant que SYSTEM (élevée), qui n'a pas accès au HKCU de l'utilisateur
// interactif — HKLM\...\Run s'applique à toute session ouverte sur la
// machine, ce qui correspond à l'usage (poste de support partagé/à distance).
// L'icône apparaîtra à la prochaine connexion, pas immédiatement : un
// processus SYSTEM ne peut pas afficher d'UI sur le bureau d'un autre
// utilisateur (isolation de session Windows).
func registerTrayAutostart(trayExePath string) error {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("ShareDeskAgentTray", `"`+trayExePath+`"`)
}
