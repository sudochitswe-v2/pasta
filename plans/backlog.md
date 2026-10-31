# Pasta Product Backlog

## 1. Enterprise & Large Network Connectivity

### Problem Statement
In large networks (such as university campuses, school dorms, corporate offices, and conference venues), devices are often placed in different subnets/VLANs (e.g., `172.26.80.x` vs `172.26.91.x`) and access points enforce **AP / Client Isolation (Station Isolation)**. This intentionally drops peer-to-peer Layer 2/Layer 3 traffic between connected Wi-Fi devices, preventing mobile devices from opening `http://<lan-ip>:8765`.

### Proposed Solutions & Features

#### A. Multi-Interface Detection & Custom Host Flag for `--qr`
- **Current Limitation:** `lanIP()` exclusively dials `8.8.8.8` to query the kernel for the primary route interface. When users set up a Wi-Fi hotspot (`10.42.0.1`), use Tailscale (`100.x.y.z`), or an HTTPS tunnel, `pasta --qr` prints the wrong IP and cannot be overridden.
- **Feature:**
  - Add `--host <ip_or_domain>` flag to `pasta` and `pasta --qr` to override the magic link host.
  - Enhance `--qr` to enumerate active non-loopback network interfaces (Wi-Fi, virtual AP/hotspots, WireGuard/Tailscale tunnels) and display selectable QR codes.

#### B. Embedded Tailscale (`tsnet`) Node (Zero-Config Mesh Connectivity)
- **Concept:** Utilize `tailscale.com/tsnet` to embed an encrypted WireGuard peer directly inside the Pasta binary.
- **Benefit:** Allows Pasta to join a private tailnet as a named machine (e.g., `pasta-host`) without requiring root access or external VPN client installations. Bypasses enterprise NAT, AP isolation, and firewalls seamlessly.

#### C. Relay / Tunneling Guides & CLI Integration
- Provide first-class documentation and optional helper scripts for:
  - **USB ADB Reverse Tethering:** `adb reverse tcp:8765 tcp:8765` for 100% offline, zero-network USB connection.
  - **Local Wi-Fi Hotspot:** `nmcli device wifi hotspot` to bypass corporate AP isolation.
  - **Zero-config HTTPS Tunnels:** Built-in recommendations or integration with Cloudflare Tunnel (`cloudflared`) or Pinggy.

---

## 2. Remote Screen Capture & Screenshot Sync (In Progress / Scoping)
- See current feature implementation plan for native X11/Windows screen capture and clipboard synchronization.
