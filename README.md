# ⚡ TailWake

**TailWake** is a lightweight, low-complexity, open-source Wake-on-LAN (WoL) HTTP daemon and Web UI designed for setups where you have an always-on machine (e.g., a Mac Mini) and target sleeping devices (e.g., a MacBook Pro), accessible securely over **Tailscale** or local network.

---

## 🎯 The Problem & Solution

### The Network Reality

Tailscale connects your devices into a secure layer-3 mesh network (Tailnet). However, standard Wake-on-LAN relies on **layer-2 UDP broadcast magic packets** (`FF:FF:FF:FF:FF:FF` + 16x MAC address) sent to `255.255.255.255`. Because VPN overlay networks do not route local ethernet broadcasts across subnets, you cannot send a WoL broadcast packet directly from your phone or remote laptop over Tailscale to wake a sleeping machine.

### The TailWake Solution

1. **Always-On Mac Mini**: Runs **TailWake** as a background HTTP daemon (~5MB RAM footprint).
2. **Remote Trigger**: When you visit the Web UI or hit `http://mini.tailscale-ip:8080/wake/macbook-pro` from your phone or laptop over Tailscale, TailWake receives the HTTP request.
3. **Local WoL Broadcast**: The Mac Mini generates the raw UDP magic packet and broadcasts it over its physical Wi-Fi/Ethernet network to your MacBook Pro.
4. **Result**: Your MacBook Pro wakes up in seconds!

---

## 🛠️ Step-by-Step Setup Instructions

### Step 1: Configure Target Machine (MacBook Pro)

To allow your MacBook Pro to wake up when receiving network packets:

1. **Enable "Wake for network access"**:
   - Open **System Settings** → **Battery** (or **Energy Saver** if plugged into power).
   - Click **Options...** (at the bottom).
   - Set **"Wake for network access"** to **Always** or **Only on AC Power**.

2. **Find your MacBook Pro's MAC Address**:
   - **Terminal Method**: Open Terminal on your MacBook Pro and run:
     ```bash
     ifconfig en0 | grep ether
     # Output: ether aa:bb:cc:dd:ee:ff
     ```
     _(If using Wi-Fi, `en0` is usually Wi-Fi on Apple Silicon. Check `ifconfig` if needed)._
   - **GUI Method**: Open **System Settings** → **Network** → Select your active connection (Wi-Fi or Ethernet) → Click **Details...** → Select **Hardware** tab → Copy **MAC Address**.

---

### Step 2: Install & Run TailWake on Mac Mini

#### Option A: Quick Installation & Launchd Daemon (Recommended)

1. Clone or copy TailWake to your Mac Mini:

   ```bash
   git clone https://github.com/kanwaloswal/TailWake.git
   cd TailWake
   ```

2. Build the binary:

   ```bash
   make build
   ```

3. Create your `config.json`:

   ```json
   {
     "port": 8080,
     "bind_address": "0.0.0.0",
     "auth_token": "",
     "devices": [
       {
         "id": "macbook-pro",
         "name": "MacBook Pro",
         "mac": "AA:BB:CC:DD:EE:FF",
         "broadcast_ip": "255.255.255.255",
         "wol_port": 9,
         "ping_host": "192.168.1.50",
         "ping_port": 22
       }
     ]
   }
   ```

   _Replace `AA:BB:CC:DD:EE:FF` with your MacBook Pro's MAC address, and `ping_host` with its local IP._

4. **Install as macOS Background Daemon (`launchd`)**:
   Run the native installer command:
   ```bash
   ./tailwake service install --config $(pwd)/config.json
   ```
   _(TailWake will now start automatically whenever your Mac Mini boots or logs in!)_

---

### Step 3: Accessing via Tailscale

Once TailWake is running on your Mac Mini, you can access it over Tailscale in two ways:

#### Method 1: Direct Tailscale IP / MagicDNS

- Open your browser on any Tailscale-connected device (iPhone, iPad, laptop) and visit:
  `http://<mac-mini-tailscale-ip>:8080` (e.g. `http://100.115.20.10:8080`) or `http://mac-mini:8080`.

#### Method 2: Tailscale Serve (Clean HTTPS URL)

- On your Mac Mini, run:
  ```bash
  tailscale serve -p 8081
  ```
- Now visit `https://mac-mini.your-tailnet.ts.net:8081` from anywhere on your Tailnet!

---

## 📱 1-Click Wake & Apple Shortcuts (iOS / macOS)

Because TailWake exposes simple GET URL triggers (`/wake/<device_id>`), you can trigger Wake-on-LAN without even opening a browser!

### Setup an Apple Shortcut (iPhone Home Screen / Siri)

1. Open the **Shortcuts** app on your iPhone or Mac.
2. Create a new Shortcut named **"Wake MacBook"**.
3. Add action: **Get Contents of URL**.
4. Enter URL: `http://<mac-mini-tailscale-ip>:8080/wake/macbook-pro` (or with token: `?token=YOUR_SECRET`).
5. Add action: **Show Notification** → _"MacBook Pro is waking up!"_.
6. Add to **Home Screen** or assign to **iPhone Action Button**!

---

## 🔒 Security Best Practices

1. **Tailscale Private Mesh Only**: By default, keep TailWake bound to your private Tailscale network or local LAN. **Do NOT run `tailscale funnel`** (which exposes ports publicly) unless token authentication is enabled.
2. **Token Authentication**: To protect against unauthorized triggers, set `"auth_token": "your_secret_key"` in `config.json` or set `TAILWAKE_TOKEN=your_secret_key`.
   - Access with token: `http://mini:8080/wake/macbook-pro?token=your_secret_key` or HTTP header `Authorization: Bearer your_secret_key`.
3. **No Shell Command Injection**: TailWake constructs raw 102-byte UDP socket frames natively in Go. It does not invoke external shell commands (`exec`), eliminating command injection vulnerabilities.

---

## 🎓 Learning Go: Code Structure Guide

If you are new to Go, TailWake is designed as a clean reference project. Here is how the codebase is organized:

```
TailWake/
├── main.go                 # Entrypoint: embedded static assets, CLI command parser
├── go.mod                  # Go module definition (zero third-party dependencies)
├── pkg/
│   ├── wol/wol.go          # Pure Go UDP Magic Packet generator (net.ParseMAC, net.DialUDP)
│   ├── status/status.go    # Device online detector using fast TCP dial probes
│   ├── config/config.go    # Structs & JSON decoder (encoding/json, os.ReadFile)
│   ├── service/service.go # macOS launchd plist generator (text/template)
│   └── web/server.go       # HTTP router & REST handlers (net/http, embed.FS)
└── web/                    # Frontend Web UI (HTML5, Vanilla CSS, JS)
```

### Key Concepts Highlighted in the Code:

- **Embedded Web UI (`go:embed`)**: The entire `web/` directory (HTML/CSS/JS) is compiled directly into the binary at build time using `//go:embed web/*`. You get a single executable file with a complete UI!
- **Pure Sockets (`net.DialUDP`)**: `pkg/wol/wol.go` demonstrates constructing a 102-byte byte array (`[102]byte`) and transmitting it directly over UDP sockets.
- **Go Routines & Mutexes (`sync.RWMutex`)**: `pkg/status/status.go` demonstrates safe concurrent memory access when checking device statuses.

---

## ⚙️ CLI Reference

```bash
# Start HTTP daemon manually
tailwake serve --config config.json --port 8080

# Send Wake-on-LAN packet directly from command line
tailwake wake macbook-pro
tailwake wake AA:BB:CC:DD:EE:FF

# Manage macOS launchd background daemon
tailwake service install --config /path/to/config.json
tailwake service uninstall

# Print version
tailwake version
```

---

## 📄 License

Apache 2.0 License.

Free and open source for everyone.
