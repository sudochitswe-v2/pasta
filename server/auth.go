package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// TokenCookieName is the session cookie set after Magic Link authentication.
const TokenCookieName = "pasta_token"

// TokenCookieMaxAge keeps the browser session alive for 30 days.
const TokenCookieMaxAge = 30 * 24 * 60 * 60

// AuthMiddleware protects every route (web UI and API) with the persistent
// daemon token. Resolution order:
//
//  1. ?token= query parameter (Magic Link from --qr).
//  2. pasta_token cookie (set after a successful query-param login).
//  3. Authorization: Bearer <token> header (API clients).
//
// A valid query-param token sets the session cookie; on GET / the request
// is then redirected to / so the token leaves browser history. Anything
// else missing/invalid gets 401 Unauthorized.
//
// An empty expectedToken disables authentication (tests and --no-auth
// style local debugging only; the daemon always configures a token).
func AuthMiddleware(expectedToken string, next http.Handler) http.Handler {
	if expectedToken == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided, viaQuery := resolveToken(r)
		if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(expectedToken)) != 1 {
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "401 Unauthorized - Invalid or missing token", http.StatusUnauthorized)
			} else {
				writeJSON(w, http.StatusUnauthorized, syncResponse{Error: "unauthorized"})
			}
			return
		}
		if viaQuery {
			setTokenCookie(w, expectedToken)
			// Scrub the token from browser history on plain page loads.
			// (API POSTs keep flowing with the query token + new cookie.)
			if r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// resolveToken returns the presented credential and whether it came from
// the query parameter.
func resolveToken(r *http.Request) (string, bool) {
	if t := r.URL.Query().Get("token"); t != "" {
		return t, true
	}
	if c, err := r.Cookie(TokenCookieName); err == nil && c.Value != "" {
		return c.Value, false
	}
	if h := r.Header.Get("Authorization"); h != "" {
		if rest, ok := strings.CutPrefix(h, "Bearer "); ok && rest != "" {
			return rest, false
		}
	}
	return "", false
}

func setTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     TokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   TokenCookieMaxAge,
		// No Secure flag: the daemon serves plain HTTP on the LAN by design.
	})
}
