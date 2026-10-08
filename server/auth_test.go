package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func authedServer() *Server {
	return New(&mockClipboard{}, &mockInjector{}, Options{RateLimit: -1, Token: testToken})
}

func TestAuthDisabledWithoutToken(t *testing.T) {
	s, _, _ := testServer(nil, &mockInjector{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (auth disabled)", rec.Code)
	}
}

func TestAuthRejectsAnonymous(t *testing.T) {
	s := authedServer()
	for _, target := range []string{"/", "/api/health", "/api/paste", "/api/type"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		if strings.HasPrefix(target, "/api/") && target != "/api/health" {
			req = httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{"text":"x"}`))
		}
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", target, rec.Code)
		}
	}
}

func TestAuthRejectsWrongToken(t *testing.T) {
	s := authedServer()
	req := httptest.NewRequest(http.MethodPost, "/api/paste?token=nope", strings.NewReader(`{"text":"x"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthQuerySetsCookie(t *testing.T) {
	s := authedServer()
	req := httptest.NewRequest(http.MethodPost, "/api/paste?token="+testToken, strings.NewReader(`{"text":"via-query"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Header.Get("Set-Cookie")
	if !strings.Contains(cookie, TokenCookieName+"="+testToken) || !strings.Contains(cookie, "HttpOnly") {
		t.Fatalf("Set-Cookie = %q, want session cookie", cookie)
	}
}

func TestAuthCookieGrantsAccess(t *testing.T) {
	s := authedServer()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: TokenCookieName, Value: testToken})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAuthBearerGrantsAccess(t *testing.T) {
	s := authedServer()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAuthQueryRedirectScrubsHistory(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // sentinel: inner handler reached
	})
	req := httptest.NewRequest(http.MethodGet, "/?token="+testToken, nil)
	rec := httptest.NewRecorder()
	AuthMiddleware(testToken, next).ServeHTTP(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", res.StatusCode)
	}
	if loc, _ := res.Location(); loc == nil || loc.Path != "/" || loc.RawQuery != "" {
		t.Fatalf("Location = %v, want bare /", loc)
	}
	if c := res.Header.Get("Set-Cookie"); !strings.Contains(c, TokenCookieName) {
		t.Fatalf("redirect missing Set-Cookie: %q", c)
	}
}

func TestAuthSubtleCompare(t *testing.T) {
	// Same length, one char off — must not pass.
	bad := testToken[:63] + "0"
	if bad == testToken {
		bad = testToken[:63] + "1"
	}
	s := authedServer()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: TokenCookieName, Value: bad})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
