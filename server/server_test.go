package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pasta/pasta/injector"
)

type mockClipboard struct {
	texts []string
	err   error
}

func (m *mockClipboard) SetText(text string) error {
	if m.err != nil {
		return m.err
	}
	m.texts = append(m.texts, text)
	return nil
}

type mockInjector struct {
	pastes int
	err    error
}

func (m *mockInjector) Paste() error {
	if m.err != nil {
		return m.err
	}
	m.pastes++
	return nil
}

func (m *mockInjector) Close() error { return nil }

func testServer(cb *mockClipboard, inj *mockInjector) (*Server, *mockClipboard, *mockInjector) {
	if cb == nil {
		cb = &mockClipboard{}
	}
	var injIface injector.Injector
	if inj != nil {
		injIface = inj
	}
	s := New(cb, injIface, Options{RateLimit: -1}) // disable rate limit
	return s, cb, inj
}

func postSync(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	req.RemoteAddr = "192.168.1.5:1234"
	rec := httptest.NewRecorder()
	s.HandleSync(rec, req)
	return rec
}

func TestSyncSuccess(t *testing.T) {
	s, cb, inj := testServer(nil, &mockInjector{})
	rec := postSync(t, s, `{"text":"hello pasta"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var resp syncResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || !resp.Injected {
		t.Fatalf("resp = %+v, want success+injected", resp)
	}
	if len(cb.texts) != 1 || cb.texts[0] != "hello pasta" {
		t.Fatalf("clipboard got %q", cb.texts)
	}
	if inj.pastes != 1 {
		t.Fatalf("pastes = %d, want 1", inj.pastes)
	}
}

func TestSyncInvalidJSON(t *testing.T) {
	s, _, _ := testServer(nil, nil)
	for _, body := range []string{"", "{", `{"text":}`, `{"nope":1}`} {
		rec := postSync(t, s, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestSyncEmptyText(t *testing.T) {
	s, cb, _ := testServer(nil, nil)
	for _, body := range []string{`{"text":""}`, `{"text":"   "}`} {
		rec := postSync(t, s, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want 400", body, rec.Code)
		}
	}
	if len(cb.texts) != 0 {
		t.Errorf("clipboard should not be touched on empty text")
	}
}

func TestSyncOversized(t *testing.T) {
	s, cb, _ := testServer(nil, nil)
	big := strings.Repeat("x", DefaultMaxTextBytes+1)
	rec := postSync(t, s, `{"text":"`+big+`"}`)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body %.80s)", rec.Code, rec.Body.String())
	}
	if len(cb.texts) != 0 {
		t.Errorf("clipboard should not be touched on oversized payload")
	}
}

func TestSyncClipboardFailure(t *testing.T) {
	s, _, _ := testServer(&mockClipboard{err: errors.New("boom")}, nil)
	rec := postSync(t, s, `{"text":"hi"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var resp syncResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Success || resp.Error == "" {
		t.Fatalf("resp = %+v, want error", resp)
	}
}

func TestSyncInjectFailureStillSucceeds(t *testing.T) {
	// Clipboard holds the text; injection is best-effort.
	s, cb, _ := testServer(nil, &mockInjector{err: errors.New("uinput denied")})
	rec := postSync(t, s, `{"text":"hi"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp syncResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Success || resp.Injected {
		t.Fatalf("resp = %+v, want success without injected", resp)
	}
	if len(cb.texts) != 1 {
		t.Fatalf("clipboard should still be set")
	}
}

func TestSyncMethodNotAllowed(t *testing.T) {
	s, _, _ := testServer(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/sync", nil)
	rec := httptest.NewRecorder()
	s.HandleSync(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestRateLimit(t *testing.T) {
	cb := &mockClipboard{}
	s := New(cb, nil, Options{RateLimit: 2, RateWindow: 60000 * 1000000})
	for i := 0; i < 2; i++ {
		if rec := postSync(t, s, `{"text":"a"}`); rec.Code != http.StatusOK {
			t.Fatalf("req %d: status = %d, want 200", i, rec.Code)
		}
	}
	if rec := postSync(t, s, `{"text":"a"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	s, _, _ := testServer(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	s.HandleHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("body = %s, want ok:true", rec.Body.String())
	}
}
