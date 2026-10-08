// Package injector simulates input in the active window via two modes:
//
// Fast Paste (Paste): writes the clipboard, then emits Ctrl+V. Fast, but
// clipboard-blocking apps and filters can ignore it.
//
// Stealth Type (Type): emits every character as raw hardware key events
// from a kernel-level virtual keyboard, bypassing user-space clipboard and
// event restrictions. Slower by design (micro-sleeps between events).
//
// Linux uses the kernel virtual-input subsystem (/dev/uinput); Windows uses
// user32.dll SendInput, optionally via a registered kernel-mode driver
// backend that strips the LLKHF_INJECTED flag. No subprocesses are spawned
// on either platform.
package injector

import "errors"

// ErrUnavailable is returned when keystroke injection is not possible
// (e.g. /dev/uinput is not writable because the user is not in the
// `input` group).
var ErrUnavailable = errors.New("injector: unavailable")

// Injector pastes or types into the focused window.
type Injector interface {
	// Paste simulates Ctrl+V in the active window (Fast Paste mode).
	Paste() error
	// Type emits text as raw hardware keystrokes (Stealth Type mode).
	Type(text string) error
	// Close releases kernel/OS resources (uinput device, handles).
	Close() error
}
