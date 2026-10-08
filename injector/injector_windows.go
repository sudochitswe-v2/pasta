//go:build windows

package injector

import (
	"errors"
	"time"
	"unsafe"

	"github.com/pasta/pasta/translator"
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
	vkShift        = 0x10
	vkReturn       = 0x0D
	vkTab          = 0x09
	vkSpace        = 0x20
)

// vkMap translates abstract translator keys to Win32 virtual-key codes.
var vkMap = map[translator.KeyCode]uint16{
	translator.KeyA: 0x41, translator.KeyB: 0x42, translator.KeyC: 0x43,
	translator.KeyD: 0x44, translator.KeyE: 0x45, translator.KeyF: 0x46,
	translator.KeyG: 0x47, translator.KeyH: 0x48, translator.KeyI: 0x49,
	translator.KeyJ: 0x4A, translator.KeyK: 0x4B, translator.KeyL: 0x4C,
	translator.KeyM: 0x4D, translator.KeyN: 0x4E, translator.KeyO: 0x4F,
	translator.KeyP: 0x50, translator.KeyQ: 0x51, translator.KeyR: 0x52,
	translator.KeyS: 0x53, translator.KeyT: 0x54, translator.KeyU: 0x55,
	translator.KeyV: 0x56, translator.KeyW: 0x57, translator.KeyX: 0x58,
	translator.KeyY: 0x59, translator.KeyZ: 0x5A,
	translator.Key1: 0x31, translator.Key2: 0x32, translator.Key3: 0x33,
	translator.Key4: 0x34, translator.Key5: 0x35, translator.Key6: 0x36,
	translator.Key7: 0x37, translator.Key8: 0x38, translator.Key9: 0x39,
	translator.Key0:     0x30,
	translator.KeyMinus: 0xBD, translator.KeyEqual: 0xBB,
	translator.KeyLeftBrace: 0xDB, translator.KeyRightBrace: 0xDD,
	translator.KeyBackslash: 0xDC, translator.KeySemicolon: 0xBA,
	translator.KeyApostrophe: 0xDE, translator.KeyGrave: 0xC0,
	translator.KeyComma: 0xBC, translator.KeyDot: 0xBE, translator.KeySlash: 0xBF,
	translator.KeySpace: vkSpace, translator.KeyEnter: vkReturn,
	translator.KeyTab: vkTab, translator.KeyShift: vkShift,
}

// DriverBackend is an optional kernel-mode keystroke backend (e.g. the
// Interception API or a Nefarius/ViGEmBus-style KMDF filter). Keystrokes
// emitted below SendInput carry LLKHF_INJECTED, which anti-cheat systems
// and secure apps use to block simulated input; a kernel driver emits them
// as genuine hardware events without that flag.
//
// Wiring: install the driver per its own installer, then call
// RegisterDriver at startup (e.g. from main) before serving. When no
// backend is registered, Type falls back to SendInput — which secure apps
// may still reject (see the LLKHF note in README).
type DriverBackend interface {
	// SendKey emits one make (down=true) / break (down=false) event for a
	// Win32 virtual-key code.
	SendKey(vk uint16, down bool) error
}

var kernelDriver DriverBackend

// RegisterDriver installs the kernel-mode backend used by Stealth Type.
// Passing nil restores the SendInput fallback.
func RegisterDriver(d DriverBackend) { kernelDriver = d }

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

func sendOne(inp input) error {
	r, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
	if r != 1 {
		if err != nil && !errors.Is(err, windows.ERROR_SUCCESS) {
			return err
		}
		return errors.New("injector: SendInput injected no input")
	}
	return nil
}

// Type implements Injector (Stealth Type mode): each character becomes raw
// key down/up events paced with micro-sleeps. With a registered kernel
// driver backend the events carry no LLKHF_INJECTED flag; otherwise
// SendInput is used and secure apps may still reject the input.
func (w *windowsInjector) Type(text string) error {
	events, err := translator.Translate(text)
	if err != nil {
		return err
	}
	for _, ev := range events {
		vk, ok := vkMap[ev.Key]
		if !ok {
			return errors.New("injector: no VK mapping")
		}
		if kernelDriver != nil {
			if err := kernelDriver.SendKey(vk, ev.Down); err != nil {
				return err
			}
		} else {
			flags := uint32(0)
			if !ev.Down {
				flags = keyeventfKeyup
			}
			if err := sendOne(input{typ: inputKeyboard, ki: keyboardInput{wVk: vk, dwFlags: flags}}); err != nil {
				return err
			}
		}
		time.Sleep(translator.DefaultKeyDelay)
	}
	return nil
}

// Close is a no-op on Windows (SendInput holds no resources).
func (w *windowsInjector) Close() error { return nil }
