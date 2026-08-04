package capture

import (
	"fmt"
	"image"

	"github.com/kbinani/screenshot"
)

// ScreenshotProvider capture l'écran principal via github.com/kbinani/screenshot
// (Win32 GDI sur Windows, X11 sur Linux, CoreGraphics sur macOS).
type ScreenshotProvider struct{}

func NewProvider() Provider { return ScreenshotProvider{} }

func (ScreenshotProvider) CaptureFrame() (image.Image, error) {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return nil, fmt.Errorf("aucun écran actif détecté")
	}
	bounds := screenshot.GetDisplayBounds(0)
	return screenshot.CaptureRect(bounds)
}
