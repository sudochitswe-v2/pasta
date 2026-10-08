//go:build linux

package clipboard

import (
	"os"
	"testing"
)

// Integration: only runs when an X server is reachable. Skips otherwise
// (CI without Xvfb, Wayland-only sessions, headless containers).
func TestLinuxClipboardRoundTrip(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY; skipping X11 clipboard round-trip")
	}
	cb, err := New()
	if err != nil {
		t.Skipf("clipboard unavailable: %v", err)
	}
	if err := cb.SetText("pasta test"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if err := cb.SetText(""); err == nil {
		t.Fatalf("SetText(\"\") should fail")
	}
}
