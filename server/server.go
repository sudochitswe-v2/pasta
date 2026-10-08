// Package server implements the pasta HTTP API.
//
// POST /api/paste accepts {"text": "..."}, writes it to the system clipboard
// via native APIs, then triggers a Ctrl+V keystroke (Fast Paste mode).
// POST /api/sync is a backward-compatible alias of /api/paste.
// POST /api/type accepts {"text": "..."} and emits it as raw hardware
// keystrokes without touching the clipboard (Stealth Type mode).
package server

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sudochitswe-v2/pasta/clipboard"
	"github.com/sudochitswe-v2/pasta/injector"
	"github.com/sudochitswe-v2/pasta/translator"
)

// Defaults for hardening (Phase 7).
const (
	// DefaultMaxTextBytes caps a single paste payload (64KB).
	DefaultMaxTextBytes = 64 * 1024
	// DefaultRateLimit caps POST /api/sync requests per client IP per window.
	DefaultRateLimit = 30
	// DefaultRateWindow is the rate-limit window.
	DefaultRateWindow = time.Minute
	// DefaultMaxTypeBytes caps a single Stealth Type payload. Typing is
	// paced at ~2ms per raw event, so large payloads take minutes; use
	// Fast Paste for bulk text and Type for short secrets.
	DefaultMaxTypeBytes = 4 * 1024
	// clipboardSettleDelay lets the clipboard propagate before Ctrl+V.
	clipboardSettleDelay = 50 * time.Millisecond
)

// Options configures a Server.
type Options struct {
	// MaxTextBytes caps payload size; <=0 means DefaultMaxTextBytes.
	MaxTextBytes int
	// MaxTypeBytes caps Stealth Type payloads; <=0 means DefaultMaxTypeBytes.
	MaxTypeBytes int
	// RateLimit caps requests per window per IP; <=0 means DefaultRateLimit.
	// Negative disables rate limiting (tests only).
	RateLimit int
	// RateWindow is the rate-limit window; <=0 means DefaultRateWindow.
	RateWindow time.Duration
	// NoInject skips keystroke injection (clipboard-only mode).
	NoInject bool
	// Token enables auth on every route when non-empty (see AuthMiddleware).
	// Empty disables auth — tests only; the daemon always sets a token.
	Token string
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
	if out.MaxTypeBytes <= 0 {
		out.MaxTypeBytes = DefaultMaxTypeBytes
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
	Typed    int    `json:"typed,omitempty"`
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

// decodeTextRequest applies the shared validation for POST text endpoints:
// rate limiting, bounded body, JSON decoding, and empty/oversize checks.
// It reports the failure response itself and returns ok=false on error.
func (s *Server) decodeTextRequest(w http.ResponseWriter, r *http.Request, maxBytes int) (string, bool) {
	if !s.allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, syncResponse{Error: "rate limit exceeded"})
		return "", false
	}
	// Bound decode memory: JSON overhead above the text limit is small.
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes)+4096)
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
		return "", false
	}
	if strings.TrimSpace(req.Text) == "" {
		writeJSON(w, http.StatusBadRequest, syncResponse{Error: "text must not be empty"})
		return "", false
	}
	if len(req.Text) > maxBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, syncResponse{Error: "text exceeds size limit"})
		return "", false
	}
	return req.Text, true
}

// HandleSync serves POST /api/paste (Fast Paste mode) and its
// backward-compatible alias POST /api/sync.
func (s *Server) HandleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, syncResponse{Error: "method not allowed"})
		return
	}
	text, ok := s.decodeTextRequest(w, r, s.opts.MaxTextBytes)
	if !ok {
		return
	}

	if err := s.clipboard.SetText(text); err != nil {
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
		s.log.Printf("sync from %s: %d bytes (injected=%v)", clientIP(r), len(text), injected)
	}
	writeJSON(w, http.StatusOK, syncResponse{Success: true, Injected: injected})
}

// HandleType serves POST /api/type (Stealth Type mode): text is emitted as
// raw hardware keystrokes without touching the clipboard.
func (s *Server) HandleType(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, syncResponse{Error: "method not allowed"})
		return
	}
	if s.injector == nil || s.opts.NoInject {
		writeJSON(w, http.StatusServiceUnavailable, syncResponse{Error: "stealth type unavailable (injection disabled)"})
		return
	}
	text, ok := s.decodeTextRequest(w, r, s.opts.MaxTypeBytes)
	if !ok {
		return
	}
	if _, err := translator.Translate(text); err != nil {
		var uerr *translator.UnsupportedError
		if errors.As(err, &uerr) {
			writeJSON(w, http.StatusBadRequest, syncResponse{Error: err.Error()})
		} else {
			writeJSON(w, http.StatusBadRequest, syncResponse{Error: "invalid text"})
		}
		return
	}
	if err := s.injector.Type(text); err != nil {
		var uerr *translator.UnsupportedError
		if errors.As(err, &uerr) {
			writeJSON(w, http.StatusBadRequest, syncResponse{Error: err.Error()})
			return
		}
		s.log.Printf("type error: %v", err)
		writeJSON(w, http.StatusInternalServerError, syncResponse{Error: "failed to type text"})
		return
	}
	if s.opts.Verbose {
		s.log.Printf("type from %s: %d runes", clientIP(r), len([]rune(text)))
	}
	writeJSON(w, http.StatusOK, syncResponse{Success: true, Injected: true, Typed: len([]rune(text))})
}

// HandleHealth serves GET /api/health for the UI connection indicator.
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, syncResponse{Error: "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Routes returns the API handler tree (static UI is served by main),
// wrapped in auth when Options.Token is set.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sync", s.HandleSync) // compat alias of /api/paste
	mux.HandleFunc("/api/paste", s.HandleSync)
	mux.HandleFunc("/api/type", s.HandleType)
	mux.HandleFunc("/api/health", s.HandleHealth)
	return AuthMiddleware(s.opts.Token, mux)
}
