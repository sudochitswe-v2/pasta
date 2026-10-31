package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
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

// loadOrCreateTLSCert ensures that cert.pem and key.pem exist in configDir,
// generating an ECDSA self-signed certificate if they do not exist.
func loadOrCreateTLSCert() (certPath, keyPath string, err error) {
	dir, err := configDir()
	if err != nil {
		return "", "", err
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	if _, errC := os.Stat(certPath); errC == nil {
		if _, errK := os.Stat(keyPath); errK == nil {
			return certPath, keyPath, nil
		}
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("tls: generate key: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("tls: serial number: %w", err)
	}

	ips := []net.IP{net.ParseIP("127.0.0.1")}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipNet.IP.To4(); ip4 != nil && !ip4.IsLoopback() {
					ips = append(ips, ip4)
				}
			}
		}
	} else if lip, err := lanIP(); err == nil {
		if parsed := net.ParseIP(lip); parsed != nil {
			ips = append(ips, parsed)
		}
	}

	dnsNames := []string{"localhost"}
	if h, err := os.Hostname(); err == nil && h != "" && h != "localhost" {
		dnsNames = append(dnsNames, h)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Pasta"},
			CommonName:   "Pasta Local Server",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           ips,
		DNSNames:              dnsNames,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", fmt.Errorf("tls: create cert: %w", err)
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", fmt.Errorf("tls: mkdir: %w", err)
	}

	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("tls: write cert: %w", err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", fmt.Errorf("tls: encode cert: %w", err)
	}

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("tls: write key: %w", err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("tls: marshal key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", fmt.Errorf("tls: encode key: %w", err)
	}

	return certPath, keyPath, nil
}
