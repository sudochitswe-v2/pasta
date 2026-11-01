#!/bin/sh
# Pasta Linux Installer
# Installs Pasta as a background systemd user service from GitHub without cloning.
#
# Quick install:
#   curl -fsSL https://raw.githubusercontent.com/sudochitswe-v2/pasta/master/install.sh | bash
#
# Custom version:
#   VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/sudochitswe-v2/pasta/master/install.sh | bash
set -eu

REPO="sudochitswe-v2/pasta"
INSTALL_DIR="${PASTA_INSTALL_DIR:-$HOME/.local/bin}"
UNIT_DIR="$HOME/.config/systemd/user"
BIN_DST="$INSTALL_DIR/pasta"
UNIT_DST="$UNIT_DIR/pasta.service"

printf "\033[1;32m==>\033[0m Installing Pasta (Remote Input Daemon)...\n"

# 1. Platform verification
OS="$(uname -s)"
ARCH="$(uname -m)"

if [ "$OS" != "Linux" ]; then
  if [ "$OS" = "Darwin" ]; then
    printf "\033[1;31merror:\033[0m macOS is not yet supported natively. Pasta requires Linux (uinput/X11) or Windows.\n" >&2
    exit 1
  else
    printf "\033[1;33mOn Windows, run this in PowerShell:\033[0m\n" >&2
    printf "  irm https://raw.githubusercontent.com/%s/master/install.ps1 | iex\n" "$REPO" >&2
    exit 1
  fi
fi

if [ "$ARCH" != "x86_64" ] && [ "$ARCH" != "amd64" ]; then
  printf "\033[1;33mwarning:\033[0m architecture %s detected; official prebuilts are compiled for x86_64/amd64.\n" "$ARCH" >&2
fi

# 2. Prepare temporary directory
TMPDIR="$(mktemp -d -t pasta-install-XXXXXX)"
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT INT TERM

# Helper to download a URL to a destination
download() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    printf "\033[1;31merror:\033[0m curl or wget is required to install Pasta.\n" >&2
    exit 1
  fi
}

# 3. Locate or download binary and service unit
PASTA_BIN=""
SERVICE_UNIT=""

# Check if running locally inside cloned repo with existing build
if [ -f "./bin/pasta" ] && [ -f "./deploy/pasta.service" ]; then
  printf "\033[1;34m-->\033[0m Using local build from current directory...\n"
  PASTA_BIN="./bin/pasta"
  SERVICE_UNIT="./deploy/pasta.service"
else
  # Remote install via GitHub
  TAG="${VERSION:-latest}"
  if [ "$TAG" = "latest" ]; then
    ARCHIVE_URL="https://github.com/${REPO}/releases/latest/download/pasta-linux-amd64.tar.gz"
  else
    ARCHIVE_URL="https://github.com/${REPO}/releases/download/${TAG}/pasta-linux-amd64.tar.gz"
  fi

  printf "\033[1;34m-->\033[0m Downloading Pasta for %s/%s...\n" "$OS" "$ARCH"

  if download "$ARCHIVE_URL" "$TMPDIR/pasta-linux-amd64.tar.gz" 2>/dev/null; then
    tar -xzf "$TMPDIR/pasta-linux-amd64.tar.gz" -C "$TMPDIR"
    PASTA_BIN="$TMPDIR/pasta"
    if [ -f "$TMPDIR/pasta.service" ]; then
      SERVICE_UNIT="$TMPDIR/pasta.service"
    fi
  else
    # Fallback to raw repository binary if release archive is not yet published
    printf "\033[1;33m-->\033[0m Release archive not found; downloading from repository master branch...\n"
    download "https://raw.githubusercontent.com/${REPO}/master/bin/pasta" "$TMPDIR/pasta"
    chmod +x "$TMPDIR/pasta"
    PASTA_BIN="$TMPDIR/pasta"
  fi
fi

# Fallback service unit definition if not already extracted
if [ -z "$SERVICE_UNIT" ] || [ ! -f "$SERVICE_UNIT" ]; then
  SERVICE_UNIT="$TMPDIR/pasta.service"
  cat <<'EOF' > "$SERVICE_UNIT"
[Unit]
Description=Pasta Remote Input Daemon
Documentation=https://github.com/sudochitswe-v2/pasta
After=graphical-session.target
PartOf=graphical-session.target

[Service]
Type=simple
ExecStart=%h/.local/bin/pasta
Restart=on-failure
RestartSec=2
Environment=DISPLAY=:0
Environment=WAYLAND_DISPLAY=wayland-0
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
EOF
fi

# 4. Install executable
mkdir -p "$INSTALL_DIR" "$UNIT_DIR"
install -m 0755 "$PASTA_BIN" "$BIN_DST"
printf "\033[1;32m✓\033[0m Installed binary to %s\n" "$BIN_DST"

# 5. Configure & install systemd service
DISPLAY_VAL="${DISPLAY:-:0}"
WAYLAND_VAL="${WAYLAND_DISPLAY:-wayland-0}"

sed -e "s|Environment=DISPLAY=.*|Environment=DISPLAY=$DISPLAY_VAL|" \
    -e "s|Environment=WAYLAND_DISPLAY=.*|Environment=WAYLAND_DISPLAY=$WAYLAND_VAL|" \
    "$SERVICE_UNIT" > "$UNIT_DST"

printf "\033[1;32m✓\033[0m Configured user service at %s\n" "$UNIT_DST"

# 6. Start the service
if command -v systemctl >/dev/null 2>&1; then
  systemctl --user daemon-reload
  systemctl --user enable --now pasta.service
  printf "\033[1;32m✓\033[0m Background service enabled and started via systemd\n"
else
  printf "\033[1;33mwarning:\033[0m systemctl not found; start manually with: %s\n" "$BIN_DST"
fi

# 7. Check /dev/uinput permissions
if ! groups | tr ' ' '\n' | grep -qx 'input'; then
  printf "\n\033[1;33m[!] Action needed for Stealth Mode (keystroke injection):\033[0m\n"
  printf "    Your user is not in the 'input' group for /dev/uinput access.\n"
  printf "    Run the following command, then log out and back in:\n"
  printf "      \033[1msudo usermod -aG input \"%s\"\033[0m\n\n" "$USER"
fi

# 8. Check PATH
case ":${PATH}:" in
  *:"$INSTALL_DIR":*) ;;
  *)
    printf "\033[1;33mNote:\033[0m %s is not in your PATH.\n" "$INSTALL_DIR"
    printf "Add it by running:\n"
    printf "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc\n\n"
    ;;
esac

# 9. Output Magic Link & QR code
printf "\n\033[1;32m====================================================\033[0m\n"
printf "\033[1;32m   Pasta successfully installed and running!        \033[0m\n"
printf "\033[1;32m====================================================\033[0m\n\n"

if [ -x "$BIN_DST" ]; then
  "$BIN_DST" --qr || true
fi
