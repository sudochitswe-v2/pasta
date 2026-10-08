// Package injector simulates a Ctrl+V keystroke in the active window.
//
// Linux uses the kernel virtual-input subsystem (/dev/uinput); Windows uses
// user32.dll SendInput. No subprocesses are spawned on either platform.
package injector

import "errors"

// ErrUnavailable is returned when keystroke injection is not possible
// (e.g. /dev/uinput is not writable because the user is not in the
// `input` group).
var ErrUnavailable = errors.New("injector: unavailable")

// Injector pastes the current clipboard contents into the focused window.
type Injector interface {
	// Paste simulates Ctrl+V in the active window.
	Paste() error
	// Close releases kernel/OS resources (uinput device, handles).
	Close() error
}
