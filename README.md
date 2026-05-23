# Watchflare

Self-hosted host monitoring. Real-time metrics, package inventory, and alerts — one binary to deploy.

[![Release](https://img.shields.io/github/v/release/watchflare-io/watchflare?label=release)](https://github.com/watchflare-io/watchflare/releases)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io-2496ED?logo=docker&logoColor=white)](https://github.com/watchflare-io/watchflare/pkgs/container/watchflare)

Watchflare collects system metrics in real time, maintains a full package inventory across your fleet, and alerts you when things go wrong — without sending your infrastructure data anywhere.

```
  your-server-1          your-server-2          your-server-3
  [ Agent ]              [ Agent ]              [ Agent ]
      |                      |                      |
      └──────────────────────┴──────────────────────┘
                             |
                         gRPC / TLS 1.3
                             |
                          [ Hub ]
                    dashboard + TimescaleDB
```

---

## Why Watchflare?

- **Zero-dependency deployment** — one Go binary embeds the entire web UI. No Nginx, no Node, no reverse proxy needed.
- **Automatic TLS** — the Hub generates its own PKI on first run. Agents pin the CA at registration. No certificate management.
- **Resilient agents** — a write-ahead log buffers metrics locally when the Hub is unreachable and replays on reconnect. No gaps.
- **Package inventory** — tracks installed packages across ~30 package managers with daily delta sync, outdated detection, and security flagging.
- **Privacy-first** — your infrastructure data never leaves your servers. AGPL-3.0 licensed.

## What it monitors

| Category | Metrics |
|----------|---------|
| **CPU** | Usage %, iowait, steal (VMs), temperature (physical hosts) |
| **Memory** | Used, available, buffers, cached, swap |
| **Disk** | Total, used, read/write throughput |
| **Network** | Inbound/outbound bandwidth |
| **System** | Uptime, load average (1/5/15 min), process count |
| **Containers** | Per-container CPU, memory, network (Docker/Podman) |
| **Packages** | Installed packages, versions, outdated detection (~30 package managers) |

---

## Quick start

**Requirements:** Docker and Docker Compose v2+.

```bash
mkdir watchflare && cd watchflare
```

Download [`docker-compose.yml`](docker-compose.yml) from this repo, then generate the three required secrets:

```bash
printf "POSTGRES_PASSWORD=%s\nJWT_SECRET=%s\nSMTP_ENCRYPTION_KEY=%s\n" \
  "$(openssl rand -hex 32)" \
  "$(openssl rand -hex 32)" \
  "$(openssl rand -hex 32)" > .env
```

Start the stack:

```bash
docker compose up -d
```

Open `http://your-host:8080`. On first load you are redirected to create your admin account.

> **Full guide** → [docs.watchflare.io/get-started/quickstart](https://docs.watchflare.io/get-started/quickstart/)

## Install an agent

In the dashboard, create a host and copy the registration token. Then run on the target machine:

**Linux:**
```bash
curl -sSL https://get.watchflare.io | sudo bash -s -- \
  --token wf_reg_YOUR_TOKEN \
  --host YOUR_HUB_IP \
  --port 50051
```

**macOS (via Homebrew):**
```bash
curl -sSL https://get.watchflare.io/brew | bash -s -- \
  --token wf_reg_YOUR_TOKEN \
  --host YOUR_HUB_IP \
  --port 50051
```

The installer registers the agent, writes the config, and starts the service. The host goes online in the dashboard within 5 seconds.

---

## Tech stack

| Component | Technology |
|-----------|------------|
| Hub | Go, Gin, gRPC, GORM |
| Frontend | SvelteKit 5, Tailwind CSS v4, uPlot |
| Database | PostgreSQL + TimescaleDB |
| Agent | Go, gopsutil |
| Security | TLS 1.3, HMAC-SHA256, JWT, bcrypt |

---

## Documentation

Full documentation at **[docs.watchflare.io](https://docs.watchflare.io)**

- [Architecture overview](https://docs.watchflare.io/get-started/architecture/)
- [Hub configuration reference](https://docs.watchflare.io/reference/hub-env/)
- [Agent install — Linux](https://docs.watchflare.io/agent/install/linux/)
- [Agent install — macOS](https://docs.watchflare.io/agent/install/macos/)
- [Alerts & notifications](https://docs.watchflare.io/monitoring/alerts-notifications/)
- [Package inventory](https://docs.watchflare.io/monitoring/packages/)

---

## Development

```bash
# 1. Start database
docker compose up -d

# 2. Hub (terminal 1)
cd backend && go run .

# 3. Frontend (terminal 2)
cd frontend && npm install && npm run dev   # http://localhost:5173
```

Copy `.env.example` to `.env` and set `JWT_SECRET` (≥ 32 chars). Default dev credentials: `admin@watchflare.io` / `watchflare_p4ss`.

See [CLAUDE.md](.claude/CLAUDE.md) for architecture notes, build commands, and contribution guidelines.

---

## License

[AGPL-3.0](LICENSE)
