# Sub2API Agent Instructions

This file is the Codex entry point for this repository. It is derived from
`CLAUDE.md`; when the two differ, prefer the more specific project rule here
or in the referenced architecture/codebase documents.

## Project Snapshot

Sub2API is an AI API gateway that aggregates multiple upstream AI subscription
accounts and exposes unified API keys. It supports token-level billing, account
scheduling/load balancing, rate limiting, circuit breaking, and two run modes:
`standard` for full SaaS billing and `simple` for internal use.

- Backend: Go 1.26.2, Gin, Ent ORM, Wire DI, PostgreSQL, Redis.
- Frontend: Vue 3, TypeScript, Vite 5, TailwindCSS, Pinia, pnpm.
- Deploy: Docker, systemd, GoReleaser.
- Upstream: `https://github.com/Wei-Shaw/sub2api`.
- Fork/origin: `https://github.com/541968679/sub2api`.

## Start Here

Before changing unfamiliar code, read these in order:

1. `docs/dev/ARCHITECTURE.md` - top-level architecture, request flow, Wire,
   Settings KV, migrations, common task templates, known pitfalls.
2. `docs/dev/codebase/README.md` - module documentation index.
3. The relevant `docs/dev/codebase/{module}.md` file if it exists.
4. `CLAUDE.md` only when you need the fuller historical context.

If deep exploration traces a flow across three or more files, update or create
the matching `docs/dev/codebase/*.md` entry using this shape:
data model -> key files -> core flow -> important mechanisms -> known pitfalls.

Update `docs/dev/ARCHITECTURE.md` only for top-level modules, cross-cutting
conventions, reusable task templates, or environment/build pitfalls.

## Key Linked Files

### Core Documentation

- `CLAUDE.md` - original project context and operating rules.
- `DEV_GUIDE.md` - local development setup and common environment issues.
- `docs/dev/ARCHITECTURE.md` - required architecture entry point.
- `docs/dev/CHANGELOG_CUSTOM.md` - custom-change log; append every verified
  local change here.
- `docs/dev/SECONDARY_DEV.md` - secondary development guide.
- `docs/dev/DEPLOYMENT.md` - deployment and operations guide.
- `docs/dev/UPSTREAM_SYNC.md` - upstream merge history.
- `docs/dev/codebase/README.md` - module map.
- `docs/dev/codebase/account.md` - account management flow.
- `docs/dev/codebase/billing.md` - pricing and billing flow.
- `docs/dev/codebase/model-mapping.md` - model whitelist/mapping flow.

### Backend Entry Points

- `backend/cmd/server/main.go` - application entry.
- `backend/cmd/server/wire.go` - Wire injection definition.
- `backend/cmd/server/wire_gen.go` - generated Wire graph; may need manual
  edits because current `go generate ./cmd/server` is known to fail on duplicate
  payment service bindings.
- `backend/internal/server/router.go` - Gin router setup.
- `backend/internal/server/routes/` - route registration by area.
- `backend/internal/handler/handler.go` - top-level handler container.
- `backend/internal/handler/wire.go` - handler provider set.
- `backend/internal/repository/wire.go` - repository provider set.
- `backend/internal/service/wire.go` - service provider set.
- `backend/internal/config/config.go` - Viper-based config loading.

### Backend Hot Paths

- `backend/internal/handler/gateway_handler.go` - core API proxy.
- `backend/internal/handler/openai_gateway_handler.go` - OpenAI-compatible
  gateway handling.
- `backend/internal/handler/gemini_v1beta_handler.go` - Gemini gateway handling.
- `backend/internal/service/gateway_service.go` - gateway orchestration.
- `backend/internal/service/account_usage_service.go` - token usage and billing.
- `backend/internal/service/setting_service.go` - public/runtime settings.
- `backend/internal/service/domain_constants.go` - settings keys and domain
  constants.
- `backend/internal/repository/setting_repo.go` - settings persistence.
- `backend/internal/repository/*_cache.go` - Redis and local cache layers.

### Data And Generated Code

- `backend/ent/schema/` - hand-written Ent schemas.
- `backend/ent/` - generated Ent code; do not hand-edit unless generated output
  is being intentionally reconciled.
- `backend/migrations/` - raw SQL migrations; Ent auto-migrate is not used.
- `backend/migrations/migrations.go` - embedded migration loader.
- `backend/resources/model-pricing/model_prices_and_context_window.json` - model
  pricing source resource.
- `backend/data/model_pricing.json` - packaged pricing data.

### Frontend Entry Points

- `frontend/package.json` - scripts and dependency truth source.
- `frontend/pnpm-lock.yaml` - must be committed; never replace with npm lockfiles.
- `frontend/vite.config.ts` - build config; frontend output embeds into backend.
- `frontend/src/main.ts` - Vue app entry.
- `frontend/src/router/index.ts` - route and auth-guard truth source.
- `frontend/src/router/README.md` - router conventions.
- `frontend/src/api/client.ts` - Axios client with auth/refresh behavior.
- `frontend/src/api/index.ts` - API exports.
- `frontend/src/stores/app.ts` - public settings cache and global UI feedback.
- `frontend/src/stores/auth.ts` - auth state.
- `frontend/src/i18n/locales/zh.ts` and `frontend/src/i18n/locales/en.ts` -
  i18n truth sources; always update both.
- `frontend/src/components/layout/` - shared app shell.
- `frontend/src/components/common/` - common UI components.
- `frontend/src/views/admin/`, `frontend/src/views/user/`,
  `frontend/src/views/auth/`, `frontend/src/views/setup/` - page views.

### Build, Deploy, And Tooling

- `Makefile` - root orchestration, but native `make` may be unavailable on
  Windows; use the underlying commands when needed.
- `backend/Makefile` - backend build/test targets.
- `Dockerfile` - multi-stage production image.
- `deploy/docker-compose.yml` - production compose stack.
- `deploy/docker-compose.dev.yml` - local development services.
- `deploy/.env.example` and `deploy/config.example.yaml` - env/config templates.
- `deploy/update.sh` - server-side update script; requires explicit user
  permission before use.
- `deploy/docker-deploy.sh` and `deploy/install.sh` - deployment/install helpers;
  require explicit user permission before use.
- `tools/check_pnpm_audit_exceptions.py` - pnpm audit exception checker.

## Development Commands

Backend:

```bash
cd backend
go run ./cmd/server/
go generate ./ent
go generate ./cmd/server
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o bin/server ./cmd/server
```

Frontend:

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm dev
pnpm build
pnpm run lint:check
pnpm run typecheck
pnpm run test
```

Root:

```bash
make build
make test
```

On Windows, prefer raw `go ...` and `pnpm ...` commands if `make` is not
available. The root `secret-scan` target currently references
`tools/secret_scan.py`, which is not present in this checkout; verify tooling
before relying on that target.

## Mandatory Workflow Rules

- Read `docs/dev/ARCHITECTURE.md` before exploring unfamiliar code.
- Append every verified change to `docs/dev/CHANGELOG_CUSTOM.md` with what
  changed, why, and affected files.
- Commit verified local changes promptly. Do not push or deploy without explicit
  user permission for that specific push/deploy.
- `docs/dev` may be ignored by Git; use `git add -f` for required doc updates.
- Use `pnpm` only. Do not use `npm`, do not create `package-lock.json`, and
  always keep `frontend/pnpm-lock.yaml` committed.
- Frontend builds into `backend/internal/web/dist` and is embedded in the Go
  binary with the `embed` build tag.
- Ent schema changes require `go generate ./ent` plus a raw SQL migration in
  `backend/migrations/`.
- Wire DI changes require updating provider sets and regenerating or manually
  reconciling `backend/cmd/server/wire_gen.go`.
- Interface changes require updating all mocks/stubs that implement the
  interface.
- New i18n keys must be added to both `frontend/src/i18n/locales/zh.ts` and
  `frontend/src/i18n/locales/en.ts`.
- Use `127.0.0.1`, not `localhost`, for local PostgreSQL on Windows.
- Do not run deployment scripts, push to `origin`, or perform production
  deployment unless the user explicitly asks for it in the current task.

## Backend Conventions

- Request flow is Gin middleware -> handler -> service -> repository ->
  Ent/PostgreSQL and Redis.
- Handlers parse/bind/authorize and return unified responses; business logic
  belongs in services.
- Runtime settings use the existing Settings KV stack:
  `domain_constants.go`, `setting_service.go`, `setting_repo.go`, public
  settings DTOs, and `GET /api/v1/settings/public`.
- Database migrations are additive where possible and must be idempotent.
- New handlers/services/repositories must be registered in the appropriate
  `wire.go` provider set.
- Generated Ent files are not a place for manual business logic.

## Frontend Conventions

- Auth and admin routing decisions belong in `frontend/src/router/index.ts`
  route meta/guards, not ad hoc page checks.
- Authenticated pages should use the shared layout components under
  `frontend/src/components/layout/`.
- Use `frontend/src/api/client.ts` and module API files; export through
  `frontend/src/api/index.ts`.
- Global success/error/warning feedback goes through `useAppStore()`.
- Public settings should be read from the app store cache and force-refetched
  after admin setting saves when needed.
- Keep UI text in i18n files; update Chinese and English together.

## Known Pitfalls

- `go generate ./cmd/server` is known to fail on current main because of
  duplicate payment service bindings. If necessary, manually edit
  `backend/cmd/server/wire_gen.go` to match the provider graph.
- Ent auto-migrate is not used; raw SQL migrations are required.
- Native Windows/Git Bash path conversion can break POSIX paths passed to
  Python/SSH. Be careful with deployment commands that pass Linux absolute paths
  through Windows shells.
- Local backend development commonly uses port `8081` because `8080` may be
  occupied by Docker.
