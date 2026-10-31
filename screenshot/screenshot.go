// Package screenshot provides platform-native screen capture functionality.
//
// On Linux, it uses native X11 APIs via CGO or Wayland screenshot tools
// (spectacle, grim, gnome-screenshot). On Windows, it uses native Win32 GDI
// syscalls (BitBlt/GetDIBits) without spawning any subprocesses.
package screenshot

import "errors"

// ErrUnavailable is returned when no graphical session or screenshot capability
// is accessible.
var ErrUnavailable = errors.New("screenshot: no graphical session or display available")

// Capturer captures the current screen and returns the image encoded as PNG.
type Capturer interface {
	CapturePNG() ([]byte, error)
}

// New constructs the platform-specific screen capturer.
func New() (Capturer, error) {
	return newCapturer()
}
