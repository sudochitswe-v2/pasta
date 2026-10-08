package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempConfigEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "pasta", "config.json")
}

func TestLoadOrCreateTokenFirstRun(t *testing.T) {
	path := tempConfigEnv(t)
	token, err := loadOrCreateToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64 hex chars", len(token))
	}
	for _, c := range token {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("token %q is not lowercase hex", token)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("config file mode = %o, want 600", info.Mode().Perm())
	}
	if mode := dirMode(t, filepath.Dir(path)); mode != 0700 {
		t.Errorf("config dir mode = %o, want 700", mode)
	}
}

func TestLoadOrCreateTokenPersists(t *testing.T) {
	tempConfigEnv(t)
	first, err := loadOrCreateToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreateToken()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("token changed across restarts: %q vs %q", first, second)
	}
}

func TestLoadOrCreateTokenCorrupt(t *testing.T) {
	path := tempConfigEnv(t)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateToken(); err == nil {
		t.Fatal("expected error for corrupt config")
	}
}

func TestMagicLink(t *testing.T) {
	link := magicLink("192.168.1.10", 8765, "abc123")
	if want := "http://192.168.1.10:8765/?token=abc123"; link != want {
		t.Fatalf("link = %q, want %q", link, want)
	}
}

func TestLanIPLooksV4(t *testing.T) {
	ip, err := lanIP()
	if err != nil {
		t.Skipf("no default route in this environment: %v", err)
	}
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		t.Fatalf("lanIP = %q, want dotted IPv4", ip)
	}
}

func TestRunQR(t *testing.T) {
	tempConfigEnv(t)
	if err := runQR(8765); err != nil {
		t.Fatalf("runQR: %v", err)
	}
}

func dirMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
