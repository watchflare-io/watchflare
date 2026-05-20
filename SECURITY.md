# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Watchflare, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please use [GitHub Security Advisories](https://github.com/watchflare-io/watchflare/security/advisories/new) to report vulnerabilities privately.

## Scope

The following are in scope:
- Watchflare backend (Go)
- Watchflare agent (Go)
- Watchflare frontend (SvelteKit)
- Docker images published on GHCR
- Installation scripts

## Security Model

Watchflare uses multiple security layers:

| Layer | Mechanism |
|-------|-----------|
| Transport | TLS 1.3 (mandatory, no fallback) |
| Agent registration | One-time token, SHA-256 hashed in DB |
| Agent authentication | HMAC-SHA256 per request |
| Anti-replay | Timestamp window (default ±5 min) |
| Web authentication | JWT in HttpOnly cookie |
| Password storage | bcrypt (cost 10) |

Full security documentation will be available at [docs.watchflare.io](https://docs.watchflare.io).
