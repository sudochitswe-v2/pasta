//go:build windows

package clipboard

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32            = windows.NewLazySystemDLL("user32.dll")
	procOpenClipboard    = modUser32.NewProc("OpenClipboard")
	procEmptyClipboard   = modUser32.NewProc("EmptyClipboard")
	procSetClipboardData = modUser32.NewProc("SetClipboardData")
	procCloseClipboard   = modUser32.NewProc("CloseClipboard")

	modKernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalAlloc   = modKernel32.NewProc("GlobalAlloc")
	procGlobalLock    = modKernel32.NewProc("GlobalLock")
	procGlobalUnlock  = modKernel32.NewProc("GlobalUnlock")
	procGlobalFree    = modKernel32.NewProc("GlobalFree")
	procRtlMoveMemory = modKernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

type windowsClipboard struct{}

// New returns a Clipboard backed by user32.dll clipboard syscalls.
func New() (Clipboard, error) {
	return &windowsClipboard{}, nil
}

// SetText implements Clipboard via OpenClipboard / EmptyClipboard /
// SetClipboardData(CF_UNICODETEXT). All calls run on a single locked OS
// thread as required by the Win32 clipboard API.
func (c *windowsClipboard) SetText(text string) error {
	if text == "" {
		return ErrEmptyText
	}
	if len(text) > MaxTextBytes {
		return errors.New("clipboard: text exceeds 64KB limit")
	}
	utf16, err := windows.UTF16FromString(text)
	if err != nil {
		return err
	}
	size := uintptr(len(utf16)) * unsafe.Sizeof(utf16[0])

	var setErr error
	done := make(chan struct{})
	go func() {
		// Clipboard APIs have thread affinity; keep everything here.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		setErr = setClipboardData(utf16, size)
	}()
	<-done
	return setErr
}

func setClipboardData(utf16 []uint16, size uintptr) error {
	r, _, _ := procOpenClipboard.Call(0)
	if r == 0 {
		return errors.New("clipboard: OpenClipboard failed (another app may hold it)")
	}
	defer procCloseClipboard.Call()

	r, _, _ = procEmptyClipboard.Call()
	if r == 0 {
		return errors.New("clipboard: EmptyClipboard failed")
	}

	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, size)
	if hMem == 0 {
		return errors.New("clipboard: GlobalAlloc failed")
	}
	// On success the system owns hMem; only free on failure paths.
	success := false
	defer func() {
		if !success {
			procGlobalFree.Call(hMem)
		}
	}()

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return errors.New("clipboard: GlobalLock failed")
	}
	// Copy UTF-16 via RtlMoveMemory (avoids uintptr->pointer round-trip).
	procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&utf16[0])), size)
	procGlobalUnlock.Call(hMem)

	r, _, _ = procSetClipboardData.Call(cfUnicodeText, hMem)
	if r == 0 {
		return errors.New("clipboard: SetClipboardData failed")
	}
	success = true
	return nil
}
