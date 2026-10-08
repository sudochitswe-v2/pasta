# Project Specification: Pasta

## 1. Overview

**Internal Project Name:** Pasta
**Application/Executable Name:** `pasta` (Linux) / `pasta.exe` (Windows)
**Objective:** A stealthy, lightweight background daemon that allows users to remotely paste text from a mobile device directly into a host's clipboard and active cursor via a local web interface.
**Target OS:** Linux (X11 / Wayland) & Windows (10/11)
**Primary Language:** Go

## 2. Stealth & Evasion Strategy

To avoid detection by background process checkers, system monitors, or battery optimization daemons, the application relies on strict operational stealth and minimal resource consumption.

### 2.1 Cross-Platform Stealth

* **Zero Subprocesses:** The daemon will **never** spawn external shell commands (like `xclip`, `wl-copy`, `clip.exe`, or `powershell.exe`) to set the clipboard or inject keys. Process spawning creates PID spikes that system monitors log immediately.

* **Event-Driven & Sleeping:** The Go runtime will park threads while waiting for HTTP requests, keeping idle CPU usage at strictly 0.0%. It avoids all polling loops.

* **Memory Profile:** Compiled as a stripped binary, expected to idle under 15MB of RAM.

### 2.2 Windows-Specific Stealth

* **EcoQoS Evasion:** Windows may throttle background apps using "Efficiency Mode". Since Pasta relies on pure event-driven socket listening, it naturally avoids heavy CPU heuristics, but can use the `SetProcessInformation` API to manage its throttling state if network latency becomes an issue.

* **Session Isolation Awareness:** The app must run in the interactive user session (Session 1+). Running as a standard Windows Service (Session 0) restricts access to the user's clipboard and active window.

## 3. Core Features

1. **Web Interface:** Serves a single-page HTML interface containing a text area and a "Send" button.

2. **Clipboard Management:** Directly interfaces with the OS clipboard via native APIs.

3. **Cursor Injection (Auto-Paste):** After setting the clipboard, the daemon simulates a `Ctrl + V` (or `Shift + Insert`) keystroke directly to the active window to paste the text instantly.

## 4. Technical Architecture

* **Language:** Go 1.21+

* **Web Server:** Go Standard Library (`net/http`) - requires no heavy external frameworks.

* **UI Delivery:** `//go:embed` to compile the HTML/CSS/JS directly into the binary.

### 4.1 Input Injection & Clipboard (Linux)

* **Clipboard:** Uses CGO bindings to natively interact with X11/Wayland.

* **Injection:** Uses direct bindings to the Linux kernel's virtual input device subsystem (`/dev/uinput`) to act as a virtual hardware keyboard, bypassing Wayland security restrictions.

* *Requirement:* The user running the daemon must be in the `input` group to write to `/dev/uinput` without requiring `sudo`.

### 4.2 Input Injection & Clipboard (Windows)

* **Clipboard:** Uses direct `syscall` to `user32.dll` (`OpenClipboard`, `EmptyClipboard`, `SetClipboardData`).

* **Injection:** Uses `user32.dll` -> `SendInput` to simulate hardware keyboard events at the OS level.

## 5. Endpoints

* `GET /`: Serves the embedded minimalist mobile web UI.

* `POST /api/sync`: Accepts a JSON payload and processes the text.

  * **Payload:** `{"text": "string"}`

  * **Behavior:** Validates payload -> Writes to system clipboard -> Triggers `Ctrl + V` via virtual input.

  * **Response:** `{"success": true}` (200 OK) or `{"error": "message"}` (500 Internal Server Error).

## 6. Deployment & Execution

The application must run seamlessly in the background as a user-level service, inheriting the user's graphical session.

### 6.1 Linux (Systemd User Service)

*Example `~/.config/systemd/user/pasta.service`:*

```
[Unit]
Description=Pasta Remote Input Daemon
After=graphical-session.target

[Service]
Type=simple
ExecStart=/home/user/.local/bin/pasta
Restart=on-failure
Environment=DISPLAY=:0
Environment=WAYLAND_DISPLAY=wayland-0

[Install]
WantedBy=default.target

```

### 6.2 Windows (Task Scheduler / Startup)

* **Execution:** Executed silently on user logon via the Registry `Run` key (`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`) or a Scheduled Task configured to run "At log on".

* **Window Hiding:** Built using `go build -ldflags -H=windowsgui` to ensure no command prompt window flashes or remains open when the daemon starts.