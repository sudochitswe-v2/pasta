package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeWebIndex(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	serveWeb(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
	body, _ := io.ReadAll(res.Body)
	for _, want := range []string{"textarea", "app.js", "style.css", "viewport"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("index.html missing %q", want)
		}
	}
}

func TestServeWebAssets(t *testing.T) {
	for path, wantCT := range map[string]string{
		"/style.css": "text/css",
		"/app.js":    "javascript",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		serveWeb(rec, req)
		res := rec.Result()
		if res.StatusCode != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, res.StatusCode)
		}
		if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, wantCT) {
			t.Errorf("%s: Content-Type = %q, want %q", path, ct, wantCT)
		}
	}
}

func TestServeWebSPAFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()
	serveWeb(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (SPA fallback)", rec.Code)
	}
}

func TestServeWebMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	serveWeb(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
