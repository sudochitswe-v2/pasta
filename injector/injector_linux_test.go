//go:build linux

package injector

import (
	"testing"
	"unsafe"

	"github.com/sudochitswe-v2/pasta/translator"
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

// Every key the translator can emit must have an evdev mapping, so Type
// never fails midway through a keystroke stream. Needs no hardware.
func TestKeyCodeMapCoversTranslator(t *testing.T) {
	s := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789`~-_=+[{]}\\|;:'\",<.>/?!@#$%^&*() \n\t\r"
	events, err := translator.Translate(s)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	seen := map[translator.KeyCode]bool{}
	for _, ev := range events {
		if seen[ev.Key] {
			continue
		}
		seen[ev.Key] = true
		if _, ok := keyCodeMap[ev.Key]; !ok {
			t.Errorf("no evdev mapping for key %d", ev.Key)
		}
	}
}
func TestUinputPasteIntegration(t *testing.T) {
	inj, err := New()
	if err != nil {
		t.Skipf("uinput unavailable: %v", err)
	}
	defer inj.Close()
	if err := inj.Paste(); err != nil {
		t.Fatalf("Paste: %v", err)
	}
	if err := inj.Type("Az09!~ \t"); err != nil {
		t.Fatalf("Type: %v", err)
	}
}
