# Sub2API - Claude Code Project Context

## Project Overview

Sub2API is an AI API gateway platform that aggregates multiple AI subscription accounts (Claude, OpenAI, Gemini, etc.) and distributes access via unified API keys. It supports token-level billing, load balancing, rate limiting, and circuit breaking.

- **Upstream**: https://github.com/Wei-Shaw/sub2api
- **License**: MIT
- **Run Modes**: `standard` (full SaaS with billing) / `simple` (internal use, no billing)

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.26.2, Gin, Ent ORM, Wire DI, gRPC |
| Frontend | Vue 3, TypeScript, Vite 5, TailwindCSS, Pinia, pnpm |
| Database | PostgreSQL 16+ |
| Cache | Redis 7+ |
| Deploy | Docker (multi-stage), systemd, GoReleaser |
| CI/CD | GitHub Actions |

## Project Structure

```
├── backend/
│   ├── cmd/server/          # Entry point (main.go, wire.go, VERSION)
│   ├── ent/                 # Ent ORM generated code & schema/
│   ├── internal/
│   │   ├── config/          # Configuration loading (Viper-based)
│   │   ├── domain/          # Core domain models
│   │   ├── handler/         # HTTP handlers (gateway, auth, admin, API keys)
│   │   ├── service/         # Business logic layer
│   │   ├── repository/      # Data access layer (200+ files, includes *_cache.go)
│   │   ├── server/          # HTTP server setup
│   │   │   └── routes/      # Route definitions (admin, auth, gateway, payment, user)
│   │   ├── middleware/       # HTTP middleware (rate limiting, recovery, CORS)
│   │   ├── payment/         # Payment providers (Stripe, WeChat, Alipay)
│   │   ├── setup/           # First-run setup wizard
│   │   ├── integration/     # Integration tests
│   │   ├── testutil/        # Test helpers
│   │   ├── pkg/             # Internal packages
│   │   ├── model/           # Data models
│   │   ├── util/            # Utilities
│   │   └── web/             # Embedded frontend serving
│   ├── migrations/          # Database migrations (Atlas-based)
│   ├── Makefile             # Backend build targets
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── api/             # Axios API client
│   │   ├── components/      # Vue components
│   │   ├── views/           # Page views (admin/, auth/, user/, setup/)
│   │   ├── stores/          # Pinia stores
│   │   ├── router/          # Vue Router
│   │   ├── i18n/            # Internationalization
│   │   ├── types/           # TypeScript definitions
│   │   └── composables/     # Vue composition functions
│   ├── package.json
│   ├── pnpm-lock.yaml       # MUST be committed
│   └── vite.config.ts       # Frontend builds to backend/internal/web/dist
├── deploy/
│   ├── docker-compose.yml   # Production (PG + Redis + App)
│   ├── docker-compose.dev.yml
│   ├── docker-deploy.sh     # One-click Docker setup
│   ├── install.sh           # Binary + systemd install
│   ├── config.example.yaml
│   └── .env.example
├── Dockerfile               # Multi-stage: node→go→alpine
├── Makefile                 # Root orchestration
├── .goreleaser.yaml         # Multi-platform release
└── DEV_GUIDE.md             # Developer setup & common pitfalls
```

## Development Commands

### Backend

```bash
cd backend
go run ./cmd/server/                    # Run dev server
go generate ./ent                       # Regenerate Ent ORM (after schema changes)
go generate ./cmd/server                # Regenerate Wire DI
go test -tags=unit ./...                # Unit tests
go test -tags=integration ./...         # Integration tests (needs PG + Redis)
golangci-lint run ./...                 # Lint check
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o bin/server ./cmd/server
```

### Frontend

```bash
cd frontend
pnpm install --frozen-lockfile          # Install deps (NEVER use npm)
pnpm dev                                # Dev server on :3000
pnpm build                              # Production build → backend/internal/web/dist
pnpm run lint:check                     # ESLint check
pnpm run typecheck                      # TypeScript check
pnpm run test                           # Vitest
```

### Root Makefile

```bash
make build                # Build frontend + backend
make test                 # Run all tests + lint
make secret-scan          # Python security scanner
```

## Key Development Rules

1. **Every change must be committed promptly** — do `git add` + `git commit` immediately after each fix/feature is verified, not batched. Push to origin after commit.
2. **Every change must be logged** — append an entry to `docs/dev/CHANGELOG_CUSTOM.md` describing what changed, why, and which files were affected.
3. **pnpm only** — never use npm. Delete `node_modules` and reinstall if mixed.
4. **pnpm-lock.yaml must be committed** — CI uses `--frozen-lockfile`.
5. **Ent schema changes** → run `go generate ./ent` and commit generated files.
6. **Wire DI changes** → run `go generate ./cmd/server` and commit `wire_gen.go`.
7. **Interface changes** → update ALL test stubs/mocks that implement the interface.
8. **Frontend embeds into backend** — `pnpm build` output goes to `backend/internal/web/dist`, compiled into the Go binary with `-tags embed`.
9. **Windows dev notes**: use `127.0.0.1` not `localhost` for psql; no Chinese paths for psql; no native `make` (use raw commands).

## Configuration

- Primary config: `config.yaml` (Viper-based, supports env var overrides)
- Docker config: environment variables in `docker-compose.yml`
- Key secrets: `JWT_SECRET`, `TOTP_ENCRYPTION_KEY` — must be fixed for persistence across restarts
- Database: PostgreSQL connection pool (`MAX_OPEN_CONNS`, `MAX_IDLE_CONNS`)
- Redis: pool size 1024 default, TLS optional

## Deployment

- **Docker Compose**: `deploy/docker-compose.yml` — app + PG + Redis, auto-setup on first run
- **Binary + systemd**: `deploy/install.sh` → installs to `/opt/sub2api` with systemd service
- **CI Release**: tag `v*` → GitHub Actions → GoReleaser → multi-arch Docker images + binaries

## Git & Repository

- **Fork (origin)**: https://github.com/541968679/sub2api — push all changes here
- **Upstream**: https://github.com/Wei-Shaw/sub2api — official repo, pull-only

## Workflow: Local Dev → Production

### 1. Start Local Environment

```bash
# Ensure Docker Desktop is running (PG + Redis containers auto-restart)
# sub2api-pg-dev (5432), sub2api-redis-dev (6379), credentials: sub2api/sub2api

# Start backend (port 8081, 8080 is occupied by Docker)
cd backend
SERVER_PORT=8081 SERVER_MODE=debug \
  DATABASE_HOST=127.0.0.1 DATABASE_PORT=5432 \
  DATABASE_USER=sub2api DATABASE_PASSWORD=sub2api \
  DATABASE_DBNAME=sub2api DATABASE_SSLMODE=disable \
  REDIS_HOST=127.0.0.1 REDIS_PORT=6379 \
  go run ./cmd/server/

# Start frontend (another terminal, proxies to backend:8081)
cd frontend
pnpm dev
# → http://localhost:3000  (admin: admin@sub2api.local / admin123456)
```

### 2. Develop & Test Locally

Edit code → Vite hot-reloads frontend automatically. Backend requires restart after Go changes.

### 3. Commit & Push to Fork

```bash
git add <changed-files>
git commit -m "fix/feat: description"
git push origin main
```

### 4. Deploy to Production Server

```bash
# Option A: Via remote_exec.py (Claude Code can run this directly)
python deploy/remote_exec.py "/opt/sub2api/update.sh"

# Option B: Manual SSH
ssh root@172.245.247.80 "/opt/sub2api/update.sh"
```

The update.sh script: pulls code → builds Docker image → restarts container → health check.

### 5. Sync Upstream (Official) Updates

```bash
git fetch upstream
git merge upstream/main    # Git preserves our custom changes
# Resolve conflicts if any, then:
git push origin main
python deploy/remote_exec.py "/opt/sub2api/update.sh"
```

Track all custom changes in `docs/dev/CHANGELOG_CUSTOM.md` and sync history in `docs/dev/UPSTREAM_SYNC.md`.

## Production Server

- **URL**: https://zerocode.kaynlab.com
- **Admin**: admin@zerocode.kaynlab.com
- **Server**: 172.245.247.80 (RackNerd, LA, Ubuntu 24.04, 5C/6G)
- **SSH**: `python deploy/remote_exec.py "<command>"` (uses deploy/remote_exec.py)
- **Stack**: Docker Compose (sub2api-custom:latest + PG 18 + Redis 8) + Caddy (auto HTTPS)
- **Firewall**: UFW, ports 22/80/443 only
- **Paths**: repo at `/opt/sub2api/repo`, compose at `/opt/sub2api/docker-compose.yml`

## Architecture Notes

- **Entry**: `backend/cmd/server/main.go` → Wire DI → Gin HTTP server with h2c
- **Request flow**: Gin middleware → handler → service → repository → Ent/PostgreSQL + Redis cache
- **Gateway**: `handler/gateway_handler.go` is the core API proxy (~66K)
- **Scheduling**: Sticky session + round-robin across upstream AI accounts
- **Circuit breaker**: 5-failure threshold, 10-min cooldown on 529 status
- **Billing**: Token-level tracking in `service/account_usage_service.go`
