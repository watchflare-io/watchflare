# Contributing to Watchflare

Thanks for your interest in contributing. Watchflare is built for the self-hosted community, and contributions of all kinds are welcome: code, documentation, bug reports, and feedback from real homelabs and production setups.

## How to contribute

- **Report a bug.** Open an [issue](https://github.com/watchflare-io/watchflare/issues/new) with your OS, Watchflare version, and steps to reproduce. Agent logs (`journalctl -u watchflare-agent` on Linux, `tail -f /var/log/watchflare-agent.log` on macOS) and Hub logs (`docker logs watchflare`) help a lot.
- **Suggest a feature.** Start a [discussion](https://github.com/watchflare-io/watchflare/discussions) first.
- **Improve the docs.** Documentation lives in the [watchflare-io/docs](https://github.com/watchflare-io/docs) repository. Every page has an "Edit this page" link.
- **Write code.** Open an issue first for anything beyond a trivial fix. New package manager collectors, OS support, and bug fixes are especially welcome.

## Development setup

**Prerequisites:** Go 1.26+, Node.js 20+, Docker and Docker Compose v2+.

```bash
git clone https://github.com/watchflare-io/watchflare.git
cd watchflare
git checkout develop
docker compose -f docker-compose-postgres.yml up -d

cp .env.example .env
# Set POSTGRES_PASSWORD, JWT_SECRET, and SMTP_ENCRYPTION_KEY to random strings.
# Generate each with: openssl rand -base64 32

# Hub (terminal 1)
cd backend && go run .

# Frontend (terminal 2)
cd frontend && npm install && npm run dev   # http://localhost:5173
```

Architecture notes at [docs.watchflare.io/get-started/architecture/](https://docs.watchflare.io/get-started/architecture/).

## Pull request guidelines

- Branch from `develop` using `<type>/<short-description>` format (`feat/discord-alerts`, `fix/scrollbar-overflow`, `docs/contributing-update`), and target it in your PR.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) in English.
- Sign off every commit with `git commit -s` to certify the [Developer Certificate of Origin](DCO). This adds a `Signed-off-by` trailer and is required for all PRs.
- Run `go test ./...` (backend, agent) and `npm run test` (frontend) before submitting.
- Check dependency vulnerabilities with `govulncheck ./...` (backend, agent) and `npm audit --omit=dev` (frontend).

## Code of conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).

## Security

Do not open public issues for security vulnerabilities. See [SECURITY.md](SECURITY.md).
