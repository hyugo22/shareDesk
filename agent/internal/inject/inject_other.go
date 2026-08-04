//go:build !windows

package inject

import "errors"

// ErrNotSupported : l'injection clavier/souris native n'est implémentée que
// pour Windows en v1 (SendInput, voir inject_windows.go). Linux (XTest) et
// macOS (CGEvent) sont le prochain point d'extension de ce package.
var ErrNotSupported = errors.New("injection clavier/souris non implémentée sur cette plateforme")

type NoopProvider struct{}

func NewProvider() Provider { return NoopProvider{} }

func (NoopProvider) MoveMouse(x, y int32) error                       { return ErrNotSupported }
func (NoopProvider) MouseButtonEvent(button MouseButton, down bool) error { return ErrNotSupported }
func (NoopProvider) MouseWheel(deltaX, deltaY int32) error             { return ErrNotSupported }
func (NoopProvider) KeyEvent(virtualKeyCode uint16, down bool) error   { return ErrNotSupported }
