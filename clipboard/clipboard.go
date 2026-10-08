// Package clipboard provides a platform-specific system clipboard writer.
//
// The zero-subprocess constraint means implementations must use native OS
// APIs only: X11 (via CGO) on Linux, user32.dll syscalls on Windows.
// Implementations must be event-driven and never poll.
package clipboard

import "errors"

// MaxTextBytes caps a single payload to prevent memory abuse.
// Mirrors server-side validation (Phase 7 hardening).
const MaxTextBytes = 64 * 1024 // 64KB

// ErrEmptyText is returned when the caller tries to set an empty clipboard.
var ErrEmptyText = errors.New("clipboard: empty text")

// ErrUnavailable is returned when no graphical session is reachable
// (e.g. DISPLAY is unset on Linux).
var ErrUnavailable = errors.New("clipboard: no graphical session available")

// Clipboard is the minimal interface the HTTP server depends on.
type Clipboard interface {
	// SetText replaces the system clipboard contents with text.
	SetText(text string) error
}
