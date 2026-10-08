// Package server implements the pasta HTTP API.
//
// POST /api/sync accepts {"text": "..."}, writes it to the system clipboard
// via native APIs, then triggers a Ctrl+V keystroke into the active window.
package server

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pasta/pasta/clipboard"
	"github.com/pasta/pasta/injector"
)

// Defaults for hardening (Phase 7).
const (
	// DefaultMaxTextBytes caps a single paste payload (64KB).
	DefaultMaxTextBytes = 64 * 1024
	// DefaultRateLimit caps POST /api/sync requests per client IP per window.
	DefaultRateLimit = 30
	// DefaultRateWindow is the rate-limit window.
	DefaultRateWindow = time.Minute
	// clipboardSettleDelay lets the clipboard propagate before Ctrl+V.
	clipboardSettleDelay = 50 * time.Millisecond
)

// Options configures a Server.
type Options struct {
	// MaxTextBytes caps payload size; <=0 means DefaultMaxTextBytes.
	MaxTextBytes int
	// RateLimit caps requests per window per IP; <=0 means DefaultRateLimit.
	// Negative disables rate limiting (tests only).
	RateLimit int
	// RateWindow is the rate-limit window; <=0 means DefaultRateWindow.
	RateWindow time.Duration
	// NoInject skips keystroke injection (clipboard-only mode).
	NoInject bool
	// Verbose enables request logging.
	Verbose bool
	// Logger receives non-request log lines; nil means log.Default().
	Logger *log.Logger
}

func (o *Options) withDefaults() Options {
	out := *o
	if out.MaxTextBytes <= 0 {
		out.MaxTextBytes = DefaultMaxTextBytes
	}
	if out.RateLimit <= 0 && o.RateLimit == 0 {
		out.RateLimit = DefaultRateLimit
	}
	if out.RateWindow <= 0 {
		out.RateWindow = DefaultRateWindow
	}
	return out
}

// Server serves the pasta API.
type Server struct {
	clipboard clipboard.Clipboard
	injector  injector.Injector
	opts      Options
	log       *log.Logger

	mu      sync.Mutex
	clients map[string]*clientState
}

type clientState struct {
	count       int
	windowStart time.Time
}

// New builds a Server. cb must be non-nil; inj may be nil (clipboard-only
// mode, e.g. when /dev/uinput is unavailable or --no-inject is set).
func New(cb clipboard.Clipboard, inj injector.Injector, opts Options) *Server {
	if cb == nil {
		panic("server: clipboard must not be nil")
	}
	o := opts.withDefaults()
	logger := o.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &Server{clipboard: cb, injector: inj, opts: o, log: logger, clients: make(map[string]*clientState)}
}

type syncRequest struct {
	Text string `json:"text"`
}

type syncResponse struct {
	Success  bool   `json:"success"`
	Injected bool   `json:"injected,omitempty"`
	Error    string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// allow checks the per-IP fixed-window rate limiter. Negative RateLimit
// disables limiting.
func (s *Server) allow(ip string) bool {
	if s.opts.RateLimit < 0 {
		return true
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.clients[ip]
	if !ok || now.Sub(st.windowStart) >= s.opts.RateWindow {
		s.clients[ip] = &clientState{count: 1, windowStart: now}
		return true
	}
	if st.count >= s.opts.RateLimit {
		return false
	}
	st.count++
	return true
}

func clientIP(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}

// HandleSync serves POST /api/sync.
func (s *Server) HandleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, syncResponse{Error: "method not allowed"})
		return
	}
	if !s.allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, syncResponse{Error: "rate limit exceeded"})
		return
	}
	// Bound decode memory: JSON overhead above the text limit is small.
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.opts.MaxTextBytes)+4096)
	var req syncRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		// http.MaxBytesReader trips first for payloads far over the limit.
		if strings.Contains(err.Error(), "too large") {
			writeJSON(w, http.StatusRequestEntityTooLarge, syncResponse{Error: "text exceeds size limit"})
		} else {
			writeJSON(w, http.StatusBadRequest, syncResponse{Error: "invalid JSON body"})
		}
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeJSON(w, http.StatusBadRequest, syncResponse{Error: "text must not be empty"})
		return
	}
	if len(req.Text) > s.opts.MaxTextBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, syncResponse{Error: "text exceeds size limit"})
		return
	}

	if err := s.clipboard.SetText(req.Text); err != nil {
		s.log.Printf("clipboard error: %v", err)
		writeJSON(w, http.StatusInternalServerError, syncResponse{Error: "failed to set clipboard"})
		return
	}

	injected := false
	if s.injector != nil && !s.opts.NoInject {
		time.Sleep(clipboardSettleDelay)
		if err := s.injector.Paste(); err != nil {
			// Clipboard already holds the text; injection is best-effort.
			s.log.Printf("inject error (clipboard still updated): %v", err)
		} else {
			injected = true
		}
	}

	if s.opts.Verbose {
		s.log.Printf("sync from %s: %d bytes (injected=%v)", clientIP(r), len(req.Text), injected)
	}
	writeJSON(w, http.StatusOK, syncResponse{Success: true, Injected: injected})
}

// HandleHealth serves GET /api/health for the UI connection indicator.
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, syncResponse{Error: "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Routes returns the API handler tree (static UI is served by main).
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sync", s.HandleSync)
	mux.HandleFunc("/api/health", s.HandleHealth)
	return mux
}
