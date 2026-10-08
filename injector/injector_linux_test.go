//go:build linux

package injector

import (
	"testing"
	"unsafe"
)

func TestUinputStructSizes(t *testing.T) {
	// Must match kernel layouts: uinput_setup = 92, input_event = 24 (amd64).
	var setup uinputSetup
	_ = setup
	if got := int(unsafe.Sizeof(uinputSetup{})); got != 92 {
		t.Fatalf("sizeof(uinputSetup) = %d, want 92", got)
	}
	if got := len(inputEvent{}.marshal()); got != 24 {
		t.Fatalf("marshalled input_event = %d bytes, want 24", got)
	}
}

func TestIoctlNumbers(t *testing.T) {
	// Values from <linux/uinput.h>, verified against system headers.
	if uiDevCreate != 0x5501 || uiDevDestroy != 0x5502 {
		t.Fatalf("UI_DEV_CREATE/DESTROY = %#x/%#x", uiDevCreate, uiDevDestroy)
	}
	if uiDevSetup != 0x405C5503 || uiSetEvbit != 0x40045564 || uiSetKeybit != 0x40045565 {
		t.Fatalf("SETUP/EVBIT/KEYBIT = %#x/%#x/%#x", uiDevSetup, uiSetEvbit, uiSetKeybit)
	}
}

// Integration: only runs when /dev/uinput is writable (user in `input` group).
func TestUinputPasteIntegration(t *testing.T) {
	inj, err := New()
	if err != nil {
		t.Skipf("uinput unavailable: %v", err)
	}
	defer inj.Close()
	if err := inj.Paste(); err != nil {
		t.Fatalf("Paste: %v", err)
	}
}
