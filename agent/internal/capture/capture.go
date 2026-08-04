// Package capture définit l'interface de capture d'écran, implémentée par
// des fichiers spécifiques à chaque plateforme (voir capture_default.go).
package capture

import "image"

type Provider interface {
	// CaptureFrame capture l'écran principal et retourne l'image brute.
	CaptureFrame() (image.Image, error)
}
