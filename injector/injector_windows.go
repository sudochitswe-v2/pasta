//go:build windows

package injector

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32     = windows.NewLazySystemDLL("user32.dll")
	procSendInput = modUser32.NewProc("SendInput")
)

const (
	inputKeyboard  = 1
	keyeventfKeyup = 0x0002
	vkControl      = 0x11
	vkV            = 0x56
)

type keyboardInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// input mirrors the Win32 INPUT struct on amd64 (40 bytes): DWORD type at
// offset 0, 4 bytes padding, then the KEYBDINPUT union member at offset 8.
// The trailing pad rounds the 28 meaningful bytes up to sizeof(INPUT) == 40.
type input struct {
	typ uint32
	pad uint32
	ki  keyboardInput
	_   [12]byte
}

type windowsInjector struct{}

// New returns an Injector backed by user32.dll SendInput.
//
// NOTE (UIPI limitation): SendInput is blocked when the foreground window
// runs elevated (as Administrator) and this daemon does not. There is no
// fix short of running elevated as well; clipboard sync still succeeds in
// that case and the user can paste manually with Ctrl+V.
func New() (Injector, error) {
	return &windowsInjector{}, nil
}

// Paste implements Injector: Ctrl down, V down, V up, Ctrl up.
func (w *windowsInjector) Paste() error {
	inputs := []input{
		{typ: inputKeyboard, ki: keyboardInput{wVk: vkControl}},
		{typ: inputKeyboard, ki: keyboardInput{wVk: vkV}},
		{typ: inputKeyboard, ki: keyboardInput{wVk: vkV, dwFlags: keyeventfKeyup}},
		{typ: inputKeyboard, ki: keyboardInput{wVk: vkControl, dwFlags: keyeventfKeyup}},
	}
	r, _, err := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if r != uintptr(len(inputs)) {
		if err != nil && !errors.Is(err, windows.ERROR_SUCCESS) {
			return errors.New("injector: SendInput failed (foreground window may be elevated — see UIPI note)")
		}
		return errors.New("injector: SendInput injected partial input")
	}
	return nil
}

// Close is a no-op on Windows (SendInput holds no resources).
func (w *windowsInjector) Close() error { return nil }
