# Pasta

A stealthy background daemon for remote text paste via a local web interface.
Send text from your phone → it lands in the host clipboard **and** is typed
into the active window via `Ctrl+V`.

- **Language:** Go 1.21+ (stdlib `net/http`, `//go:embed` UI, one dep: `golang.org/x/sys`)
- **Platforms:** Linux (X11 via CGO, `/dev/uinput` injector) & Windows 10/11 (`user32.dll` syscalls)
- **Constraint:** zero subprocess spawning — all OS interaction through native APIs

## Quick start

```sh
make build-linux        # -> bin/pasta  (stripped)
./bin/pasta --port 8765 # serves UI on http://<lan-ip>:8765
```

Open the printed URL on your phone, type, hit **Send**.

```powershell
make build-windows      # -> bin\pasta.exe (stripped, no console window)
```

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--port` | `8765` | TCP port |
| `--bind` | `0.0.0.0` | Bind address — use `127.0.0.1` to restrict to this host |
| `--no-inject` | off | Clipboard-only mode (skip `Ctrl+V` injection) |
| `--max-bytes` | `65536` | Max paste payload in bytes |
| `--verbose` | off | Request logging (stderr only — **no log files ever**) |

## API

- `GET /` — embedded mobile web UI (dark, thumb-friendly, `Ctrl+Enter` to send)
- `GET /api/health` → `{"ok":true}` (UI connection indicator)
- `POST /api/sync` with `{"text":"…"}` → `{"success":true,"injected":true}`
  - `400` empty/invalid JSON · `413` over size limit · `429` rate-limited (30 req/min/IP) · `500` clipboard failure

## Install as a background service

**Linux (systemd user service):**

```sh
sudo usermod -aG input "$USER"  # required for /dev/uinput auto-paste; then re-login
./deploy/install.sh             # installs bin/pasta + enables pasta.service
```

**Windows (run at logon, no console window):**

```powershell
powershell -ExecutionPolicy Bypass -File deploy\install.ps1
```

## Security notes

- Default `--bind 0.0.0.0` exposes the server to your LAN: anyone on the
  network can write to your clipboard and inject keystrokes. Use only on
  trusted networks, or bind `127.0.0.1` + SSH tunnel.
- Windows UIPI: `SendInput` is blocked into **elevated** (Administrator)
  windows unless pasta also runs elevated. Clipboard sync still succeeds —
  paste manually with `Ctrl+V` in that case.
- Pure-Wayland sessions (no `DISPLAY`): run under XWayland; `/dev/uinput`
  requires membership in the `input` group.

## Stealth profile (measured)

- Idle RSS ≈ 12 MB (< 15 MB target), 0:00 CPU when idle (all threads park in
  `accept`/`XNextEvent` — no polling)
- Stripped binary (`-ldflags="-s -w"`), `-H=windowsgui` on Windows (no console)
- No child processes, no log files, graceful `SIGTERM`/`SIGINT` shutdown

## Development

```sh
make test   # unit + integration (X11/uinput tests skip gracefully w/o display)
make vet
make run    # go run . --verbose
```

## Layout

```
main.go  server/  clipboard/  injector/  web/  deploy/  Makefile
```
