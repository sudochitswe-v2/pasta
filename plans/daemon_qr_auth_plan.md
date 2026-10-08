# Implementation Plan: Background Daemon with Dynamic CLI QR Generation

This document outlines the step-by-step implementation plan for the changes detailed in `changelog.md` concerning the daemon's architecture, authentication, and QR generation.

## 1. Persistent Authentication Token Management
**Goal:** Ensure secure, persistent authentication across sessions without hardcoded passwords or ephemeral tokens.

*   **Config Structure:** Create a JSON configuration structure containing the token.
*   **Storage Location:** Determine the user config directory (using `os.UserConfigDir()`). Store the configuration file at `~/.config/pasta/config.json`.
*   **Initialization Flow:** 
    *   On startup, attempt to read `config.json`.
    *   If the file exists and contains a valid token, load it into memory.
    *   If it does not exist (first run), securely generate a cryptographically random 32-byte token (using `crypto/rand`), encode it in hex, ensure the directory `~/.config/pasta` exists with `0700` permissions, and save `config.json` with `0600` permissions.

## 2. CLI QR Generator (`--qr` / `--setup`)
**Goal:** Allow users to dynamically generate a QR code with a Magic Link for their current Wi-Fi network without interfering with the running daemon.

*   **Flag Addition:** Add a boolean flag `--qr` (and optionally `--setup` as an alias) in `main.go`.
*   **Execution Flow:**
    *   If `--qr` is passed, skip starting the HTTP server, clipboard initialization, and injector initialization.
    *   Load or generate the persistent token as described above.
    *   **IP Detection:** Create a helper function to resolve the active LAN IP by simulating a UDP dial to an external address (e.g., `net.Dial("udp", "8.8.8.8:80")`). This is much more reliable than enumerating network interfaces.
    *   **Magic Link Construction:** Construct the URL `http://<LAN_IP>:<PORT>/?token=<TOKEN>`.
    *   **QR Code Rendering:** Add a dependency such as `github.com/mdp/qrterminal/v3`. Use it to render the QR code for the constructed Magic Link directly to `os.Stdout`.
    *   Exit with status `0`.

## 3. Server Authentication Middleware
**Goal:** Protect all endpoints (web UI and API) utilizing the persistent token.

*   **Middleware Implementation:** Create a generic HTTP middleware function (e.g., `authMiddleware(expectedToken string, next http.Handler) http.Handler`).
*   **Token Resolution Logic:** The middleware should check for the token in the following order:
    1.  **Query Parameter:** `?token=<TOKEN>` (Used by the Magic Link).
    2.  **Cookie:** A cookie named `pasta_token`.
    3.  **Authorization Header:** `Authorization: Bearer <TOKEN>` (For future-proofing API usage).
*   **Stateful Sessions:**
    *   If the token is provided via the query parameter and is valid, the middleware should respond by setting a `Set-Cookie` header (`pasta_token=<TOKEN>; Path=/; HttpOnly; Max-Age=...`) to ensure subsequent API requests from the web UI are automatically authenticated.
    *   Optionally, redirect the user from `/?token=...` to `/` to remove the token from the browser history, or just let the client handle it.
*   **Rejection:** If the token is missing or invalid, respond with a `401 Unauthorized` status.
*   **Integration:** Wrap the main `http.ServeMux` or individual routes in `main.go` and `server/server.go` with this middleware.

## 4. Daemon Binding Verification
**Goal:** Ensure the background daemon is reachable across changing network interfaces.

*   **Binding:** `main.go` already defaults the `--bind` flag to `0.0.0.0`. Ensure this remains untouched, so the daemon listens on all interfaces.
*   **Logging:** When starting the server in daemon mode, ensure the startup log clearly states it is bound to `0.0.0.0` but instruct the user to use the `--qr` flag to get the current connection details.

## 5. Web UI Updates
**Goal:** Ensure the client successfully passes authentication.

*   If using the cookie approach described above, `fetch` requests in `web/app.js` will automatically include the cookie (ensure no `credentials: 'omit'` is set). No major frontend changes are required.

## Implementation Order
1.  Add `config.go` to handle generating and saving the token.
2.  Add the authentication middleware and apply it to `server.go` and `serveWeb`.
3.  Add the `--qr` flag logic and the IP detection helper in `main.go`.
4.  Run `go get github.com/mdp/qrterminal/v3` and implement the terminal QR output.
5.  Test the flow: First run without QR, generate QR, scan/open URL, verify cookie generation, and test API endpoints.
