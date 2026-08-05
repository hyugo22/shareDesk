//go:build !windows

package main

// registerTrayAutostart : l'icône de zone de notification n'est fournie que
// pour Windows pour l'instant (voir agent/cmd/tray) — no-op ailleurs.
func registerTrayAutostart(trayExePath string) error {
	return nil
}
