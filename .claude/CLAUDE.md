# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Watchflare: self-hosted host monitoring stack. **Hub** (Go, gRPC + HTTP), **Agent** (Go), **Frontend** (SvelteKit 5, SSE).
Data flow: Agents → gRPC/TLS 1.3 → Hub → PostgreSQL/TimescaleDB → SSE → Frontend

## Versioning

**Commit format (Conventional Commits, English):**
- `feat: description`: new feature
- `fix: description`: bug fix
- `chore:`, `docs:`, `refactor:`, `test:`, `ci:`, `style:`: maintenance

Scope is optional: `feat(agent): description`, `fix(hub): description`.

**Branch naming:** `<type>/<short-description>` matching the Conventional Commits prefix (`feat/discord-alerts`, `fix/scrollbar-overflow`, `docs/contributing-update`). Branch from and target `develop`.

**DCO sign-off required:** every commit must have `Signed-off-by:` (use `git commit -s`). The DCO text is in the `DCO` file at the repo root. The GitHub DCO App enforces this on PRs.

**Release process:** see `docs/release-workflow.md`

## Build & Run

### Hub
```bash
cd backend
go run .                                            # Dev
go build -o watchflare-backend                      # Hub binary only (no frontend embedded)
go build -tags embed_frontend -o watchflare-app     # Hub + embedded frontend (production)
go test ./...                           # Tests (uses in-memory SQLite)
go test ./handlers -v                   # Single package
go test -run TestCreateAgent ./services # Single test
```
Env: copy `.env.example` to `.env`, set `POSTGRES_PASSWORD`, `JWT_SECRET` (>=32 chars), and `NOTIFICATION_ENCRYPTION_KEY` (>=32 chars). Generate each with `openssl rand -base64 32`. On first launch, the Hub redirects to create an admin account.

### Agent
```bash
cd agent
go build -o watchflare-agent            # Build (always use -o flag)
go test ./...                           # Tests
./watchflare-agent register --token=wf_reg_... --host=localhost --port=50051
```

### Frontend
```bash
cd frontend
npm install && npm run dev              # Dev (http://localhost:5173)
npm run build                           # Production build
npm run test                            # Vitest
```

### Database
```bash
docker compose -f docker-compose-postgres.yml up -d   # Start TimescaleDB only
docker exec -it watchflare-postgres psql -U watchflare -d watchflare
```
Connection: `postgresql://watchflare:watchflare_dev@localhost:5432/watchflare` (default port from `.env.example`).

### Dev session
1. `docker compose -f docker-compose-postgres.yml up -d` → 2. `cd backend && go run .` → 3. `cd frontend && npm run dev`

## Architecture (Key Decisions)

- **Heartbeats**: 5s agent → in-memory cache (no DB write) → SSE broadcast. DB sync every 5min. Stale after 15s.
- **SSE minification**: metric fields compressed to 1-2 chars in `backend/sse/broker.go`, decoded in `frontend/src/lib/sse.js`. Both must be updated together.
- **TimescaleDB continuous aggregates**: 10m/15m/2h/8h buckets for time ranges. Migrations embedded via `//go:embed`
- **Agent security**: runs as unprivileged `watchflare` user. HMAC-SHA256 per RPC, ±5min timestamp window
- **WAL**: append-only metrics buffer when backend unreachable, auto-replay on reconnect
- **Clock desync**: detected in gRPC interceptor, tracked in HeartbeatCache, shown as frontend banner

## Critical Patterns

- **Protobuf**: schema in `shared/proto/agent.proto`, generated Go code in `shared/proto/` (run `buf generate` or `protoc` to regenerate)
- **New RPC**: protobuf message must have `agent_id`, `agent_key`, `timestamp` fields for HMAC auth
- **New metric field**: update `backend/sse/broker.go` (minify) + `frontend/src/lib/sse.js` (decode)
- **New migration**: never modify existing files in `backend/database/migrations/`, create new numbered file
- **New package collector**: implement `Collector` interface in `agent/packages/`, register in `registry.go`
- **Frontend components**: Svelte 5 runes (`$props`, `$state`, `$derived`), bits-ui for dropdowns/selects

## Security Rules

- Tokens/keys: never log, never return in API responses
- File permissions: 0600 keys, 0640 configs
- HMAC: always `hmac.Equal()` (constant-time), never `==`
- TLS 1.3: `MinVersion` and `MaxVersion` both `VersionTLS13`
- Registration tokens: SHA-256 hashed before DB storage

## Key Entry Points

| Component | File | Purpose |
|-----------|------|---------|
| Hub bootstrap | `backend/main.go` | HTTP + gRPC + 3 background workers |
| Agent bootstrap | `agent/main.go` | register vs run mode |
| gRPC handlers | `backend/grpc/agent_service.go` | Register, Heartbeat, SendMetrics, SendPackageInventory |
| HTTP handlers | `backend/handlers/` | auth, hosts, metrics, packages, sse |
| Metrics loop | `agent/wal/sender.go:Run()` | Collect → WAL → Send |
| Cache | `backend/cache/heartbeat.go` | In-memory heartbeat state |
| SSE broker | `backend/sse/broker.go` | Event broadcasting |

## Tests and vulnerability checks

Before submitting any change, run:
- `cd backend && go test ./... && govulncheck ./...`
- `cd agent && go test ./... && govulncheck ./...`
- `cd frontend && npm run test && npm audit --omit=dev`

CI runs the same checks on every PR (`.github/workflows/vulncheck.yml`).

## Strategic constraints (non-negotiable)

- **AGPL-3.0 forever.** No Community Edition vs Enterprise Edition split. The Hub code is never modified to add paywalled features.
- **DCO only, no CLA.** Contributions are accepted under AGPL-3.0 only. No relicensing.
- **Cloud monetization plans are private.** When implemented, Cloud features live in a separate repo and talk to the Hub via its public API (satellite architecture). Never propose modifying the Hub to support proprietary Cloud features.
- **Notifications library:** `github.com/nicholas-fedor/shoutrrr` (fork) once integrated. Do not propose alternatives without strong reason.

Full strategic context in `docs/private/business-strategy.md` (gitignored, local only).

## Documentation

- `README.md`: project intro and screenshots
- `CONTRIBUTING.md`: contribution workflow (DCO, branch naming, PR rules)
- `CODE_OF_CONDUCT.md`: Contributor Covenant 3.0
- `DCO`: Developer Certificate of Origin (verbatim, do not modify)
- `SECURITY.md`: security disclosure policy
- `docs/` (local, gitignored): architecture deep dives, internals, install guides, version history
- `docs/private/` (local, gitignored): strategic and business documents
- `.claude/rules/`: detailed supplementary rules for AI agents (architecture, code style, testing, security, agent paths)
- [`watchflare-io/docs`](https://github.com/watchflare-io/docs): separate GitHub repo for public user-facing documentation, published at [docs.watchflare.io](https://docs.watchflare.io)
