// Package inject définit l'interface d'injection clavier/souris, implémentée
// par plateforme. v1 fournit une implémentation native pour Windows
// (SendInput, sans CGO) ; Linux (XTest) et macOS (CGEvent) sont de simples
// points d'extension pour l'instant (voir inject_other.go).
package inject

type MouseButton int

const (
	MouseLeft MouseButton = iota
	MouseRight
	MouseMiddle
)

type Provider interface {
	MoveMouse(x, y int32) error
	MouseButtonEvent(button MouseButton, down bool) error
	MouseWheel(deltaX, deltaY int32) error
	KeyEvent(virtualKeyCode uint16, down bool) error
}
