# Tink

**English** | [简体中文](README.zh-CN.md)

> **Self-hosted, macOS-first HTTP notification gateway**

Tink is a lightweight, self-hosted notification gateway that relays HTTP notifications from any system (CI/CD, monitoring, shell scripts, cron jobs, webhooks, etc.) to all of your Mac devices with low latency and high reliability.

---

## 🌟 Core Features

- **Minimal architecture**: Go server + bbolt (single-file embedded database) + SSE (Server-Sent Events). No external database required.
- **Multi-device by design**: follows a `User → Device → Connection` model — one notification is automatically fan-out to all Macs (MacBook, iMac, Mac mini…) of a user, or can be delivered to selected devices.
- **Reliable offline replay**: auto-incrementing `uint64` message IDs plus the SSE `Last-Event-ID` mechanism guarantee lossless catch-up of notifications after reconnection.
- **macOS client (macOS 15+, Swift 6 / SwiftUI)**:
  - Native `MenuBarExtra` menu-bar presence plus a multi-column management window.
  - Local notifications via `UNUserNotificationCenter`, clicking a notification opens the target URL.
  - Automatic reconnection with exponential backoff and random jitter.
  - Built-in UI localization (English / Simplified Chinese, follows system language).
  - Credential safety: token and device UUID stored in the macOS Keychain.
- **Web dashboard**: embedded Svelte console for managing devices, secret keys, pushes testing, and settings (visit `/dashboard`).
- **Bark integration**: forward pushes to mobile through `bark_devices`, or run Tink as a fully transparent Bark server proxy (`/bark-relay`).
- **Convenient tooling**: `tink send`-style CLI helper script and one-liner `just`/`task` commands.

---

## 📦 Project Structure

```text
tink/
├── VERSION                 # Single source of truth for the release version, read by build scripts
├── server/                 # Go backend service
│   ├── cmd/server/         # CLI entrypoint (serve / token / device subcommands)
│   ├── internal/
│   │   ├── api/            # REST API handlers & middleware (auth, rate-limiting, CORS)
│   │   ├── auth/           # Bearer token verification & hashed storage
│   │   ├── device/         # Device registry
│   │   ├── i18n/           # Localized API messages (EN / zh-Hans)
│   │   ├── message/        # Message push & Bark forwarding
│   │   ├── sse/            # In-memory SSE hub with heartbeat keep-alive
│   │   ├── store/          # bbolt message store, indexes, offline replay, auto pruning
│   │   └── web/            # go:embed layer for the compiled dashboard assets
│   ├── web/                # Svelte web dashboard (Vite + TypeScript)
│   └── go.mod              # Go module (github.com/mrasong/tink/server)
├── client/                 # macOS client (Swift Package Manager, Swift 6.2)
│   ├── Package.swift
│   └── Sources/Tink/
│       ├── App/            # MenuBarExtra app entry
│       ├── Features/       # MenuBar / Settings / Windows views
│       ├── Models/         # Data models
│       ├── Resources/      # xcstrings localizations, icons
│       ├── Services/       # Network (REST + SSE), Notifications, Storage (Keychain)
│       └── Shared/         # Shared UI components & helpers
├── scripts/
│   ├── build_server.sh     # Compile Go server → build/tink-server
│   ├── build_app.sh        # Bundle macOS Tink.app (universal)
│   ├── build_docker.sh     # Build & push multi-arch Docker images
│   └── send.sh             # curl wrapper for quick pushes
├── Dockerfile              # Container image build
├── docker-compose.yml      # Docker Compose deployment
├── justfile                # `just` task runner commands
├── Taskfile.yml            # `task` runner commands
└── docs/                   # API reference, database schema (EN / zh-CN)
```

---

## ⚡ Quick Commands (Just / Task)

With [`just`](https://github.com/casey/just) or [`task`](https://taskfile.dev/) installed:

```bash
just build          # Build server + client   (or: task build)
just run-server     # Run the server           (or: task run-server)
just run-client     # Build & open Tink.app    (or: task run-client)
just test           # Run backend tests        (or: task test)

# Send a test notification
just send title="Hello" body="World"          # or: task send TITLE="Hello" BODY="World"

# Token & device management (works online or directly against the offline data file)
just token-list
just token-create name="ci-key"
just device-list
```

The version is maintained solely in the root `VERSION` file; the build ID is the 7-char short hash of the latest Git commit, injected automatically:

```bash
./scripts/build_server.sh
./scripts/build_app.sh
./scripts/build_docker.sh
```

Pass arguments to override defaults for one-off builds, e.g. `./scripts/build_server.sh 0.3.0`. The second argument of `build_app.sh` overrides the numeric bundle build; for the server it overrides the build ID. A third argument to `build_app.sh` overrides the displayed Git commit, e.g. `./scripts/build_app.sh 0.3.0 2026091411 abc1234`.

---

## 🚀 Server Quick Start

### 1. Build & run locally

```bash
# Option A: via script
./scripts/build_server.sh

# Option B: manually
cd server && go build -o ../build/tink-server ./cmd/server && cd ..

# Run (default port 5021; the initial Admin Secret Key is printed to the console on first boot)
./build/tink-server serve --port 5021 --data ./data
```

Then open `http://localhost:5021/dashboard` (or simply `http://localhost:5021`) in a browser, log in with the Admin Secret Key, and you are in the web console — manage devices and secret keys, or fire test pushes.

> **Custom dashboard route**: set the `TINK_DASHBOARD_ROUTE` environment variable or the `--dashboard-route` flag (e.g. `/my-panel`) to move the console to a private path. Once set, both `/` and `/dashboard` return 404, so the path cannot be probed.

### 2. Docker Compose (recommended)

```bash
docker compose up -d
```

### Prebuilt images

CI publishes multi-arch (amd64/arm64) images to GHCR on every release:

```bash
docker pull ghcr.io/mrasong/tink:latest   # or a version tag, e.g. v0.9.30
```

---

## 🖥️ macOS Client

Prebuilt drag-to-install DMGs (arm64 / x86_64) and universal zips are attached to each [GitHub Release](releases).

Requires macOS 15+ and Swift 6.2+ to build from source.

```bash
# Build a bundled Tink.app (recommended)
./scripts/build_app.sh

# Or run directly via Swift Package Manager
cd client && swift run Tink
```

On first launch, open Settings, enter your server URL and Bearer token — the device registers itself and connects in real time. Tokens and the device UUID are kept in the Keychain.

---

## 📡 API Examples (cURL)

### 1. Send a notification (desktop via Tink + mobile via Bark in one call)

```bash
curl -X POST http://localhost:5021/api/v1/messages \
  -H "Authorization: Bearer sk-tink-your_token" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Database Backup",
    "body": "Daily backup completed successfully.",
    "url": "https://dashboard.example.com",
    "sound": "default",
    "devices": ["your-tink-device-id"],
    "bark_devices": ["your-bark-device-key"],
    "bark_params": {
      "level": "timeSensitive",
      "icon": "https://example.com/icon.png",
      "badge": 1
    }
  }'
```

> Note: broadcast-without-target is disabled — at least one of `devices` (Tink clients) or `bark_devices` (Bark clients) is required. `bark_params` is an optional key/value map appended when forwarding to the upstream Bark server (`level`, `icon`, `badge`, `copy`, `autoCopy`, …). These keys reach the iOS Bark app only — macOS notifications always show the Tink app icon, because macOS offers no per-notification icon API (see *Notification icon (macOS)* in [docs/apidoc.md](docs/apidoc.md)).
> Enable **Bark Relay** in the web console and configure the upstream URL, and every `bark_devices` entry is transparently forwarded to the upstream Bark server. Point the iOS Bark App at `http://your-domain:5021/bark-relay` to use Tink as a fully native Bark proxy (device registration and `device_key` issuing included).

### 2. Health check

```bash
curl http://localhost:5021/api/v1/ping
```

Full interface documentation: [docs/apidoc.md](docs/apidoc.md) ([中文](docs/apidoc.zh-CN.md)) · Database schema: [docs/database_schema.md](docs/database_schema.md) ([中文](docs/database_schema.zh-CN.md))

---

## 📄 License

[MIT](LICENSE)
