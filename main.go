// Command pasta is a stealthy background daemon for remote text paste via
// a local web interface.
//
// It serves an embedded mobile-first web UI, writes received text to the
// system clipboard through native OS APIs (zero subprocesses), and
// simulates Ctrl+V in the active window.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
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

	"github.com/sudochitswe-v2/pasta/clipboard"
	"github.com/sudochitswe-v2/pasta/injector"
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
	)
	flag.Parse()

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

	srv := server.New(cb, inj, server.Options{
		MaxTextBytes: *maxBytes,
		MaxTypeBytes: *maxTypeBytes,
		NoInject:     *noInject,
		Verbose:      *verbose,
	})

	mux := http.NewServeMux()
	mux.Handle("/api/", srv.Routes())
	mux.HandleFunc("/", serveWeb)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
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

	if *bind == "0.0.0.0" {
		fmt.Printf("pasta: listening on http://<lan-ip>:%d (bound to all interfaces; see README security notes)\n", *port)
	} else {
		fmt.Printf("pasta: listening on http://%s\n", addr)
	}
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
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
