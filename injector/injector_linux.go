//go:build linux

package injector

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctl request numbers from <linux/uinput.h> (_IO/_IOW with base 'U').
// Verified against system headers; see development_plan.md Phase 3.
const (
	uiDevCreate  = 0x5501
	uiDevDestroy = 0x5502
	uiDevSetup   = 0x405C5503
	uiSetEvbit   = 0x40045564
	uiSetKeybit  = 0x40045565
)

// Event / key codes from <linux/input-event-codes.h>.
const (
	evSyn       = 0x00
	evKey       = 0x01
	synReport   = 0
	keyLeftCtrl = 29
	keyV        = 47
)

// uinputSetup mirrors `struct uinput_setup` (88 + 4 = 92 bytes, no padding
// on amd64: input_id[8] + name[80] + ff_effects_max[4]).
type uinputSetup struct {
	Bustype      uint16
	Vendor       uint16
	Product      uint16
	Version      uint16
	Name         [80]byte
	FFEffectsMax uint32
}

// inputEvent mirrors `struct input_event` on amd64 (24 bytes:
// timeval[16] + type[2] + code[2] + value[4]).
type inputEvent struct {
	Sec   int64
	Usec  int64
	Type  uint16
	Code  uint16
	Value int32
}

func (e inputEvent) marshal() []byte {
	b := make([]byte, 24)
	binary.LittleEndian.PutUint64(b[0:8], uint64(e.Sec))
	binary.LittleEndian.PutUint64(b[8:16], uint64(e.Usec))
	binary.LittleEndian.PutUint16(b[16:18], e.Type)
	binary.LittleEndian.PutUint16(b[18:20], e.Code)
	binary.LittleEndian.PutUint32(b[20:24], uint32(e.Value))
	return b
}

type uinputInjector struct {
	mu     sync.Mutex
	f      *os.File
	closed bool
}

// New creates the /dev/uinput virtual keyboard.
//
// Returns a descriptive error when /dev/uinput is not writable — usually
// because the user is not in the `input` group. Callers should treat this
// as non-fatal and continue in clipboard-only mode.
func New() (Injector, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: cannot write /dev/uinput (add yourself to the `input` group: `sudo usermod -aG input $USER`, then re-login): %v", ErrUnavailable, err)
		}
		return nil, fmt.Errorf("%w: cannot open /dev/uinput: %v", ErrUnavailable, err)
	}
	inj := &uinputInjector{f: f}
	if err := inj.setup(); err != nil {
		f.Close()
		return nil, err
	}
	return inj, nil
}

func ioctl(fd uintptr, req uint, arg uintptr) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(req), arg)
	if errno != 0 {
		return errno
	}
	return nil
}

func (u *uinputInjector) setup() error {
	fd := u.f.Fd()
	if err := ioctl(fd, uiSetEvbit, uintptr(evKey)); err != nil {
		return fmt.Errorf("injector: UI_SET_EVBIT failed: %w", err)
	}
	if err := ioctl(fd, uiSetKeybit, uintptr(keyLeftCtrl)); err != nil {
		return fmt.Errorf("injector: UI_SET_KEYBIT(LEFTCTRL) failed: %w", err)
	}
	if err := ioctl(fd, uiSetKeybit, uintptr(keyV)); err != nil {
		return fmt.Errorf("injector: UI_SET_KEYBIT(V) failed: %w", err)
	}
	var setup uinputSetup
	setup.Bustype = 0x03 // BUS_USB
	setup.Vendor = 0xCAFE
	setup.Product = 0x9A57
	copy(setup.Name[:], "Pasta virtual keyboard")
	if err := ioctl(fd, uiDevSetup, uintptr(unsafe.Pointer(&setup))); err != nil {
		return fmt.Errorf("injector: UI_DEV_SETUP failed: %w", err)
	}
	if err := ioctl(fd, uiDevCreate, 0); err != nil {
		return fmt.Errorf("injector: UI_DEV_CREATE failed: %w", err)
	}
	// Give the kernel a moment to register the device before first use.
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (u *uinputInjector) emit(evType, code uint16, value int32) error {
	now := time.Now()
	ev := inputEvent{Sec: now.Unix(), Usec: int64(now.Nanosecond() / 1000), Type: evType, Code: code, Value: value}
	if _, err := u.f.Write(ev.marshal()); err != nil {
		return err
	}
	syn := inputEvent{Sec: now.Unix(), Usec: int64(now.Nanosecond() / 1000), Type: evSyn, Code: synReport, Value: 0}
	_, err := u.f.Write(syn.marshal())
	return err
}

// Paste implements Injector by emitting: Ctrl down, V down, V up, Ctrl up.
func (u *uinputInjector) Paste() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return errors.New("injector: device closed")
	}
	seq := []struct {
		typ   uint16
		code  uint16
		value int32
	}{
		{evKey, keyLeftCtrl, 1},
		{evKey, keyV, 1},
		{evKey, keyV, 0},
		{evKey, keyLeftCtrl, 0},
	}
	for _, s := range seq {
		if err := u.emit(s.typ, s.code, s.value); err != nil {
			return fmt.Errorf("injector: write to /dev/uinput failed: %w", err)
		}
	}
	return nil
}

// Close destroys the virtual device and releases the file descriptor.
func (u *uinputInjector) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return nil
	}
	u.closed = true
	_ = ioctl(u.f.Fd(), uiDevDestroy, 0)
	return u.f.Close()
}
