// Command pasta is a stealthy background daemon for remote text paste via
// a local web interface.
//
// It serves an embedded mobile-first web UI, writes received text to the
// system clipboard through native OS APIs (zero subprocesses), and
// simulates Ctrl+V in the active window.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/sudochitswe-v2/pasta/clipboard"
	"github.com/sudochitswe-v2/pasta/injector"
	"github.com/sudochitswe-v2/pasta/screenshot"
	"github.com/sudochitswe-v2/pasta/server"
)

//go:embed web/index.html web/style.css web/app.js
var webFS embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pasta:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		port         = flag.Int("port", 8765, "TCP port to listen on")
		bind         = flag.String("bind", "0.0.0.0", "address to bind (use 127.0.0.1 to restrict to this host)")
		verbose      = flag.Bool("verbose", false, "enable request logging (no log files are written)")
		noInject     = flag.Bool("no-inject", false, "clipboard-only mode: skip Ctrl+V keystroke injection")
		maxBytes     = flag.Int("max-bytes", server.DefaultMaxTextBytes, "max paste payload in bytes")
		maxTypeBytes = flag.Int("max-type-bytes", server.DefaultMaxTypeBytes, "max stealth-type payload in bytes")
		tlsMode      = flag.Bool("tls", false, "enable HTTPS with auto-generated self-signed certificate (required for Clipboard API on iOS Safari)")
		qr           = flag.Bool("qr", false, "print a Magic Link QR for this LAN and exit (no daemon)")
		setup        = flag.Bool("setup", false, "alias of --qr")
	)
	flag.Parse()

	if *qr || *setup {
		return runQR(*port, *tlsMode)
	}

	token, err := loadOrCreateToken()
	if err != nil {
		return err
	}

	logger := log.New(os.Stderr, "pasta: ", log.LstdFlags)
	if !*verbose {
		logger.SetOutput(discardWriter{})
	}
	_ = logger // verbose logger; errors below always print.

	addr := net.JoinHostPort(strings.TrimSpace(*bind), fmt.Sprint(*port))

	cb, err := clipboard.New()
	if err != nil {
		return fmt.Errorf("clipboard init: %w", err)
	}
	fmt.Printf("pasta: clipboard ready\n")

	var inj injector.Injector
	if *noInject {
		fmt.Printf("pasta: injection disabled (--no-inject), clipboard-only mode\n")
	} else {
		inj, err = injector.New()
		if err != nil {
			// Non-fatal: clipboard sync still works, user pastes manually.
			fmt.Printf("pasta: injector unavailable, clipboard-only mode: %v\n", err)
			inj = nil
		} else {
			fmt.Printf("pasta: injector ready\n")
		}
	}
	if inj != nil {
		defer inj.Close()
	}

	capturer, err := screenshot.New()
	if err != nil {
		fmt.Printf("pasta: screenshot capturer unavailable: %v\n", err)
		capturer = nil
	} else {
		fmt.Printf("pasta: screenshot capturer ready\n")
	}

	srv := server.New(cb, inj, server.Options{
		MaxTextBytes: *maxBytes,
		MaxTypeBytes: *maxTypeBytes,
		NoInject:     *noInject,
		Verbose:      *verbose,
		Token:        token,
		Capturer:     capturer,
	})

	mux := http.NewServeMux()
	mux.Handle("/api/", srv.Routes())
	mux.HandleFunc("/", serveWeb)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           server.AuthMiddleware(token, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown on SIGTERM/SIGINT so the uinput device is destroyed.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Printf("\npasta: shutting down\n")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}()

	certFile, keyFile, err := loadOrCreateTLSCert()
	var tlsCfg *tls.Config
	if err == nil {
		if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
			tlsCfg = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
		}
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	var srvLn net.Listener = ln
	if tlsCfg != nil {
		srvLn = &dualListener{Listener: ln, tlsConfig: tlsCfg}
		if *bind == "0.0.0.0" {
			fmt.Printf("pasta: listening on 0.0.0.0:%d (HTTP + HTTPS) — run `pasta --qr` for the Magic Link\n", *port)
		} else {
			fmt.Printf("pasta: listening on http://%s and https://%s\n", addr, addr)
		}
	} else {
		if *bind == "0.0.0.0" {
			fmt.Printf("pasta: listening on 0.0.0.0:%d (HTTP) — run `pasta --qr` for the Magic Link\n", *port)
		} else {
			fmt.Printf("pasta: listening on http://%s\n", addr)
		}
	}

	if err := httpSrv.Serve(srvLn); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

type sniffConn struct {
	net.Conn
	reader io.Reader
}

func (s *sniffConn) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

type dualListener struct {
	net.Listener
	tlsConfig *tls.Config
}

func (l *dualListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	header := make([]byte, 1)
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := io.ReadFull(c, header)
	_ = c.SetReadDeadline(time.Time{})

	if err != nil {
		c.Close()
		return nil, err
	}

	combined := &sniffConn{
		Conn:   c,
		reader: io.MultiReader(bytes.NewReader(header[:n]), c),
	}

	// 0x16 is the TLS handshake record type (ClientHello)
	if header[0] == 0x16 {
		return tls.Server(combined, l.tlsConfig), nil
	}

	// Plain HTTP connection
	return combined, nil
}

var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "application/javascript; charset=utf-8",
}

// serveWeb serves the embedded UI. Only the three known assets exist;
// everything else maps to index.html (single-page app).
func serveWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	switch name {
	case "", "index.html":
		name = "index.html"
	case "style.css", "app.js":
	default:
		name = "index.html"
	}
	data, err := fs.ReadFile(webFS, "web/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if ct, ok := contentTypes[path.Ext(name)]; ok {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// lanIP resolves the active LAN address by asking the kernel which source
// address it would use toward the internet. No packets are sent; this only
// needs a default route, and beats enumerating interfaces (Docker bridges,
// VPNs, down links).
func lanIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("qr: cannot determine LAN IP (no default route?): %w", err)
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("qr: unexpected local address %T", conn.LocalAddr())
	}
	return addr.IP.String(), nil
}

// magicLink builds the authenticated URL encoded in the --qr output.
func magicLink(ip string, port int, token string, useTLS ...bool) string {
	scheme := "http"
	if len(useTLS) > 0 && useTLS[0] {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d/?token=%s", scheme, ip, port, token)
}

// runQR prints the Magic Link QR for the current network and exits without
// touching the clipboard, injector, or HTTP server.
func runQR(port int, useTLS ...bool) error {
	token, err := loadOrCreateToken()
	if err != nil {
		return err
	}
	ip, err := lanIP()
	if err != nil {
		return err
	}
	tlsEnabled := len(useTLS) > 0 && useTLS[0]
	link := magicLink(ip, port, token, tlsEnabled)
	if tlsEnabled {
		fmt.Printf("pasta: HTTPS Magic Link (iOS Safari 1-tap clipboard):\n%s\n\n", link)
	} else {
		fmt.Printf("pasta: Magic Link (same Wi-Fi):\n%s\n\n", link)
		httpsLink := magicLink(ip, port, token, true)
		fmt.Printf("pasta: HTTPS Link (for iOS Safari 1-tap clipboard):\n%s\n\n", httpsLink)
	}
	qrterminal.GenerateWithConfig(link, qrterminal.Config{
		Level:      qrterminal.M,
		Writer:     os.Stdout,
		HalfBlocks: true,
	})
	return nil
}
