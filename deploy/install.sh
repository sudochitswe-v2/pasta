#!/bin/sh
# Pasta Linux installer: copies the binary to ~/.local/bin and enables the
# systemd user service. Idempotent; safe to re-run.
set -eu

BIN_SRC="${1:-./bin/pasta}"
BIN_DST="$HOME/.local/bin/pasta"
UNIT_SRC="./deploy/pasta.service"
UNIT_DST="$HOME/.config/systemd/user/pasta.service"

if [ ! -f "$BIN_SRC" ]; then
  echo "error: binary not found at $BIN_SRC (run 'make build-linux' first)" >&2
  exit 1
fi

mkdir -p "$HOME/.local/bin" "$HOME/.config/systemd/user"
install -m 0755 "$BIN_SRC" "$BIN_DST"
echo "installed $BIN_DST"

# Prefer the actual session displays when present.
DISPLAY_VAL="${DISPLAY:-:0}"
WAYLAND_VAL="${WAYLAND_DISPLAY:-wayland-0}"
sed -e "s|Environment=DISPLAY=.*|Environment=DISPLAY=$DISPLAY_VAL|" \
    -e "s|Environment=WAYLAND_DISPLAY=.*|Environment=WAYLAND_DISPLAY=$WAYLAND_VAL|" \
    "$UNIT_SRC" > "$UNIT_DST"
echo "installed $UNIT_DST (DISPLAY=$DISPLAY_VAL WAYLAND_DISPLAY=$WAYLAND_VAL)"

if ! groups | tr ' ' '\n' | grep -qx 'input'; then
  echo "warning: you are not in the 'input' group; auto-paste needs /dev/uinput." >&2
  echo "  run: sudo usermod -aG input \"\$USER\"  (then log out and back in)" >&2
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl --user daemon-reload
  systemctl --user enable --now pasta.service
  echo "service enabled and started (systemctl --user status pasta)"
else
  echo "systemctl not found; start manually: $BIN_DST"
fi
