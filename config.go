package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// tokenBytes is the raw entropy per persistent auth token (32 bytes,
// hex-encoded to 64 characters in config.json).
const tokenBytes = 32

// fileConfig is the on-disk JSON structure at ~/.config/pasta/config.json.
type fileConfig struct {
	Token string `json:"token"`
}

// configDir returns the pasta config directory (~/.config/pasta on Linux,
// %AppData%\pasta on Windows).
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: cannot locate user config dir: %w", err)
	}
	return filepath.Join(base, "pasta"), nil
}

// loadOrCreateToken implements the persistent-auth initialization flow: read
// config.json and return its token, or generate a cryptographically random
// token on first run and persist it (dir 0700, file 0600).
func loadOrCreateToken() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "config.json")

	if data, err := os.ReadFile(path); err == nil {
		var fc fileConfig
		if err := json.Unmarshal(data, &fc); err != nil {
			return "", fmt.Errorf("config: corrupt %s: %w", path, err)
		}
		if fc.Token == "" {
			return "", fmt.Errorf("config: %s contains an empty token", path)
		}
		return fc.Token, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("config: cannot read %s: %w", path, err)
	}

	// First run: generate and persist.
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("config: cannot generate token: %w", err)
	}
	token := hex.EncodeToString(raw)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("config: cannot create %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(fileConfig{Token: token}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("config: cannot encode config: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return "", fmt.Errorf("config: cannot write %s: %w", path, err)
	}
	return token, nil
}
