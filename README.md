# Pasta

A stealthy background daemon for remote text paste via a local web interface.
Send text from your phone → it lands in the host via **Fast Paste**
(clipboard + `Ctrl+V`) or **Stealth Type** (raw hardware keystrokes that
bypass clipboard-blocking and secure apps).

- **Language:** Go 1.21+ (stdlib `net/http`, `//go:embed` UI, one dep: `golang.org/x/sys`)
- **Platforms:** Linux (X11 via CGO, `/dev/uinput` injector) & Windows 10/11 (`user32.dll` syscalls, optional KMDF driver backend)
- **Constraint:** zero subprocess spawning — all OS interaction through native APIs

## Quick start

```sh
make build-linux        # -> bin/pasta  (stripped)
./bin/pasta --port 8765 # serves UI on http://<lan-ip>:8765
```

Open the printed URL on your phone, pick a mode, type, hit **Send**.

```powershell
make build-windows      # -> bin\pasta.exe (stripped, no console window)
```

## First run: pairing your phone (`--qr`)

The daemon is locked by a persistent token (no passwords). On first run it
generates a random 32-byte token at `~/.config/pasta/config.json`
(dir `0700`, file `0600`) and reuses it forever after.

```sh
./bin/pasta --qr            # (or --setup) prints a Magic Link + terminal QR, then exits
```

Scan the QR (or open the link) on the same Wi-Fi. The link
`http://<lan-ip>:8765/?token=…` sets an `HttpOnly` session cookie and
redirects to `/`, so the token leaves browser history and later API calls
just work. API clients can also send `Authorization: Bearer <token>`.
Without a valid token every route (UI included) returns `401`.

## Modes

| Mode | Endpoint | How it works | Best for |
|---|---|---|---|
| **Fast Paste** | `POST /api/paste` (`/api/sync` alias) | Clipboard + `Ctrl+V` | Bulk text, instant |
| **Stealth Type** | `POST /api/type` | Per-character raw key events from a kernel virtual keyboard (2ms micro-sleeps) | Passwords, secure apps, anti-cheat, clipboard blockers |

Stealth Type never touches the clipboard and supports printable ASCII
(letters, digits, US punctuation, space/enter/tab). Unsupported characters
fail loudly with `400` naming the character — no silent drops.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--port` | `8765` | TCP port |
| `--bind` | `0.0.0.0` | Bind address — use `127.0.0.1` to restrict to this host |
| `--no-inject` | off | Clipboard-only mode (disables `Ctrl+V` and `/api/type` → `503`) |
| `--max-bytes` | `65536` | Max paste payload in bytes |
| `--max-type-bytes` | `4096` | Max stealth-type payload (typing is paced ~2ms/event) |
| `--qr`, `--setup` | off | Print the Magic Link QR for this LAN and exit (no daemon) |
| `--verbose` | off | Request logging (stderr only — **no log files ever**) |

## API

- `GET /` — embedded mobile web UI (mode toggle, `Ctrl+Enter` to send)
- `GET /api/health` → `{"ok":true}` (UI connection indicator)
- `POST /api/paste` with `{"text":"…"}` → `{"success":true,"injected":true}`
  - `400` empty/invalid JSON · `413` over size limit · `429` rate-limited (30 req/min/IP) · `500` clipboard failure
- `POST /api/sync` — backward-compatible alias of `/api/paste`
- `POST /api/type` with `{"text":"…"}` → `{"success":true,"injected":true,"typed":8}`
  - `400` unsupported character · `413` over 4KB · `503` injection disabled · `500` device failure

## Linux Prerequisites (for `/dev/uinput` auto-paste)

To allow the daemon to emit virtual keyboard keystrokes (`Ctrl+V`) without root:

1. **Load the `uinput` kernel module:**
   ```sh
   sudo modprobe uinput
   echo "uinput" | sudo tee /etc/modules-load.d/uinput.conf   # auto-load on boot
   ```

2. **Add your user to the `input` group:**
   ```sh
   sudo usermod -aG input "$USER"
   ```

3. **Ensure udev permissions rule is configured:**
   ```sh
   echo 'KERNEL=="uinput", GROUP="input", MODE="0660"' | sudo tee /etc/udev/rules.d/99-uinput.rules
   sudo udevadm control --reload-rules && sudo udevadm trigger
   ```
   *Note: Log out and log back in (or run `newgrp input`) for group membership to take effect.*

---

## Firewall Setup (Allow Mobile Phone Access)

If your host runs a firewall such as `ufw`, open port `8765` so your mobile device can reach the web UI:

- **Local Wi-Fi subnet only (recommended):**
  ```sh
  sudo ufw allow from 192.168.1.0/24 to any port 8765 proto tcp comment "Pasta local"
  sudo ufw reload
  ```
- **Allow port from any network:**
  ```sh
  sudo ufw allow 8765/tcp comment "Pasta"
  sudo ufw reload
  ```

---

## Install as a background service

### Linux (systemd user service)

```sh
make build-linux
./deploy/install.sh
```

This installs the binary to `~/.local/bin/pasta`, configures `~/.config/systemd/user/pasta.service` with your session display (`DISPLAY` / `WAYLAND_DISPLAY`), and enables it to run automatically on login.

#### Managing the Service

| Action | Command |
|---|---|
| **Check service status** | `systemctl --user status pasta` |
| **View live logs** | `journalctl --user -u pasta -f` |
| **Stop service** | `systemctl --user stop pasta` |
| **Start / Resume service** | `systemctl --user start pasta` |
| **Restart service** | `systemctl --user restart pasta` |
| **Disable auto-start on login** | `systemctl --user disable pasta` |
| **Stop and disable completely** | `systemctl --user disable --now pasta` |
| **Re-enable and start** | `systemctl --user enable --now pasta` |

#### Uninstall (Linux)

```sh
systemctl --user disable --now pasta
rm -f ~/.config/systemd/user/pasta.service ~/.local/bin/pasta
systemctl --user daemon-reload
```

---

### Windows (run at logon, no console window)

```powershell
powershell -ExecutionPolicy Bypass -File deploy\install.ps1
```

## Security notes

- LAN exposure: `--bind 0.0.0.0` (default) listens on all interfaces, but
  every route now requires the token — anonymous requests get `401`. Still,
  use trusted networks only (traffic is plain HTTP).
- Token storage: `~/.config/pasta/config.json`, `0600`. Anyone with the
  token (or your unlocked phone session) can inject keystrokes.
- Windows UIPI: `SendInput` is blocked into **elevated** (Administrator)
  windows unless pasta also runs elevated. Clipboard sync still succeeds —
  paste manually with `Ctrl+V` in that case.
- Windows LLKHF: Stealth Type without a kernel driver still carries the
  `LLKHF_INJECTED` flag, which some anti-cheat/secure apps filter. For full
  bypass, install the [Interception](https://github.com/oblitum/Interception)
  driver (or a Nefarius/ViGEmBus-style KMDF filter) with its own installer,
  then wire it via `injector.RegisterDriver()` at startup — the
  `DriverBackend` interface (`SendKey(vk, down)`) is the integration point.
  Without it, pasta uses the `SendInput` fallback and says so in logs.
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
main.go  server/  clipboard/  injector/  translator/  web/  deploy/  Makefile
```
