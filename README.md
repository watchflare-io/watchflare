# Watchflare

Self-hosted infrastructure monitoring with real-time dashboards. Lightweight agents report metrics over gRPC/TLS to a central Hub, distributed as a single binary with an embedded web UI.

![Dashboard](docs/screenshots/dashboard.png)
*↑ screenshot coming soon*

## Why Watchflare?

- **Zero-dependency deployment** — one Go binary embeds the entire frontend. No Nginx, no Node, no reverse proxy required.
- **Automatic TLS** — the Hub generates its own PKI on first run. Agents pin the CA on registration. No certificate management.
- **Resilient by design** — agents buffer metrics locally (WAL) when the Hub is unreachable and replay on reconnect. No gaps.
- **Package inventory** — tracks installed packages across 28 package managers (apt, brew, pip, npm, cargo, …) with daily delta detection and security/outdated flagging.
- **Privacy-first** — your infrastructure data never leaves your servers. AGPL-3.0 licensed.

## Features

- **Real-time monitoring** — CPU, memory, disk, network, load average, temperature via SSE streaming
- **Docker/Podman container metrics** — per-container CPU, memory, network tracking
- **Lightweight agents** — single binary, ~10 MB, runs as a system service (Linux + macOS)
- **Secure by default** — TLS 1.3 (auto-generated PKI), HMAC-signed RPCs, JWT authentication
- **Package inventory** — 28 package managers, daily delta sync, security advisory flagging
- **Write-ahead log** — agents buffer metrics locally when the Hub is unreachable
- **Alert rules** — configurable thresholds with SMTP notifications
- **Incident history** — per-host and global incident timeline
- **TimescaleDB** — automatic partitioning, compression, continuous aggregates, 30-day retention

## Screenshots

| Dashboard | Host detail | Packages |
|-----------|-------------|----------|
| ![Dashboard](docs/screenshots/dashboard.png) | ![Host detail](docs/screenshots/host-detail.png) | ![Packages](docs/screenshots/packages.png) |

*Screenshots coming soon*

## Quick Start

### Requirements

- Docker + Docker Compose (recommended)
- Or: Linux/macOS host with PostgreSQL + TimescaleDB
- **Hub:** ~50 MB RAM, any modern CPU
- **Agent:** ~10 MB RAM, Linux (x86\_64/arm64) or macOS (x86\_64/arm64)

### Docker Compose (recommended)

```bash
git clone https://github.com/watchflare-io/watchflare.git
cd watchflare

# Configure environment
cp .env.example .env
# Edit .env: set POSTGRES_PASSWORD and JWT_SECRET

# Start
docker compose up -d

# Open http://localhost:8080 and create your admin account
```

### Pre-built binaries

Download the latest release from [GitHub Releases](https://github.com/watchflare-io/watchflare/releases):

| Binary | Platform |
|--------|----------|
| `watchflare-app` | Hub (Linux x86\_64 / arm64, macOS) |
| `watchflare-agent` | Agent (Linux x86\_64 / arm64, macOS) |

### Install an agent

After creating a host in the dashboard, copy the registration token and run:

```bash
curl -sSL https://get.watchflare.io | sudo bash -s -- \
  --token=wf_reg_xxx --host=your-hub-address --port=50051
```

### Development

```bash
# Start database
docker compose up -d

# Hub (terminal 1)
cd backend
cp .env.example .env
go run .

# Frontend (terminal 2)
cd frontend
npm install
npm run dev
```

## Tech Stack

| Component | Technology |
|-----------|------------|
| Hub | Go, Gin, gRPC, GORM |
| Frontend | SvelteKit 5, Tailwind CSS v4, uPlot |
| Database | PostgreSQL + TimescaleDB |
| Agent | Go, gopsutil |
| Security | TLS 1.3, HMAC-SHA256, JWT, bcrypt |

## Documentation

Full documentation at [docs.watchflare.io](https://docs.watchflare.io) — coming soon.

## License

AGPL-3.0 — see [LICENSE](LICENSE) for details.
