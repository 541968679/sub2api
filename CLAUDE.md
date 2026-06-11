# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Read this chain first

1. `AGENTS.md` — repository-local agent rules, dev-stack entrypoints, local ports, and sibling-repo boundaries.
2. `docs/dev/ARCHITECTURE.md` — top-level request flow, Wire DI, Settings KV, migrations, and known pitfalls.
3. `docs/dev/codebase/README.md` — module map.
4. The relevant `docs/dev/codebase/{module}.md` file before changing a subsystem.
5. `docs/dev/RELATED_PROJECTS.md` when the task touches AIClient2API, InvokeAI, `new-api`, shared ports, or cross-repo contracts.

If you trace a flow across 3+ files, update the corresponding `docs/dev/codebase/*.md`. Update `docs/dev/ARCHITECTURE.md` only for cross-cutting conventions, new top-level modules, reusable patterns, or environment/build pitfalls.

## Development commands

### Normal local start/stop

Use the repo script for normal local work; it manages backend, frontend, and optional sibling services.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 restart
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 status
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 stop
scripts\dev-stack.cmd restart
```

Useful flags:

```powershell
# Start only Sub2API, skip AIClient2API
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 restart -SkipAIClient

# Also include the sibling new-api integration on 127.0.0.1:13200
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 restart -IncludeNewAPI
```

Default local endpoints:

- Backend: `http://127.0.0.1:18081`
- Frontend: `http://127.0.0.1:15174`
- PostgreSQL: `127.0.0.1:5432`
- Redis: `127.0.0.1:6379`

Avoid local ports `8080`, `8081`, `5173`, `5174`, and `5175`; this checkout reserves `18081` and `15174` instead.

### Backend

Run from `backend/` unless using the `pnpm --dir` style from repo root.

```bash
go generate ./ent
go generate ./cmd/server
go test ./...
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o bin/server ./cmd/server
```

Single-package / single-test patterns:

```bash
go test -tags=unit ./internal/service -run TestSettingService_GetPublicSettings -count=1
go test -tags=unit ./internal/handler/dto -run TestPublicSettingsInjectionPayload_SchemaDoesNotDrift -count=1
```

Notes:

- `backend/Makefile` exposes `build`, `generate`, `test`, `test-unit`, `test-integration`, and `test-e2e`.
- Wire output is generated from `backend/cmd/server/wire.go` into `wire_gen.go`.
- `go generate ./cmd/server` is currently known to fail on duplicate payment service bindings; if it does, reconcile `backend/cmd/server/wire_gen.go` manually to match the provider graph.

### Frontend

Use `pnpm` only.

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend build
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend test
pnpm --dir frontend test:run
pnpm --dir frontend test:coverage
```

Single-spec / single-case patterns:

```bash
pnpm --dir frontend exec vitest run src/views/auth/__tests__/LinuxDoCallbackView.spec.ts
pnpm --dir frontend exec vitest run src/views/auth/__tests__/LinuxDoCallbackView.spec.ts -t "renders callback result"
```

Frontend build output goes to `backend/internal/web/dist` and is embedded into the Go binary for production.

### Root orchestration

```bash
make build
make test
```

Important nuance: root `make test` does **not** run the full frontend Vitest suite. It runs backend tests, frontend lint, frontend typecheck, and only the critical spec list defined in the root `Makefile` (`FRONTEND_CRITICAL_VITEST`). For a broader frontend check, run `pnpm --dir frontend test:run` directly.

On Windows, native `make` may be unavailable; use the underlying `go ...` and `pnpm ...` commands instead.

## High-level architecture

### Product shape

Sub2API is an AI API gateway that multiplexes upstream accounts (Claude, OpenAI, Gemini, Antigravity, etc.) behind user API keys. The two main run modes are:

- `standard` — full SaaS behavior with billing/quota enforcement
- `simple` — internal mode with billing and quota checks disabled

### Backend bootstrap and request flow

The backend entry is `backend/cmd/server/main.go`.

- Startup first decides between setup wizard mode and normal server mode.
- Normal server bootstraps configuration, logger, and the full dependency graph through Wire (`backend/cmd/server/wire.go`, generated `wire_gen.go`).
- `backend/internal/server/router.go` attaches logging, CORS, security headers, embedded-frontend middleware, and then delegates route registration to `backend/internal/server/routes/`.

The main runtime path is:

```text
Gin middleware -> handler -> service -> repository -> Ent/PostgreSQL + Redis
```

Handlers should stay thin: parse/bind/auth/response. Business behavior lives in services. Data access and caching live in repositories.

### Route families and protocol adapters

The route layout is split by both product area and upstream protocol:

- `GET /api/v1/settings/public`, `/api/v1/auth/*`, `/api/v1/admin/*`, `/api/v1/...` user/payment/admin routes
- `/v1/*` — Anthropic-compatible gateway surface plus OpenAI-compatible endpoints routed by group platform
- `/v1beta/*` — Gemini native API compatibility
- Root aliases like `/responses`, `/chat/completions`, `/embeddings`, `/images/*`, and `/backend-api/codex/*`
- `/antigravity/v1/*` and `/antigravity/v1beta/*` — Antigravity-specific surfaces

The important big-picture split is in `backend/internal/server/routes/gateway.go`:

- API-key middleware authenticates the key and resolves the key's active group/platform.
- The gateway then dispatches requests to either `GatewayHandler` or `OpenAIGatewayHandler` depending on platform and endpoint.
- This is why Anthropic-style, OpenAI-style, Gemini, Codex, and Antigravity requests can coexist on one deployment while still sharing billing, scheduling, and error reporting infrastructure.

### Gateway hot path

The gateway is the core of the product, and the non-obvious flow spans routes, handlers, services, and repositories:

- `backend/internal/handler/gateway_handler.go` and `openai_gateway_handler.go` are the HTTP-layer protocol adapters.
- `backend/internal/service/gateway_service.go` and OpenAI-specific gateway services orchestrate upstream request construction, sticky session behavior, failover, and response streaming.
- `backend/internal/service/account_usage_service.go` records token usage and billing effects.
- Redis-backed caches and scheduler state live in `backend/internal/repository/*_cache.go`.

When debugging routing problems, always inspect the route file, handler, service, and any scheduler/cache helper together; behavior is spread across those layers by design.

### Dependency injection and generated code

Wire is the assembly mechanism for nearly everything in the backend.

- Provider sets live in `backend/internal/repository/wire.go`, `backend/internal/service/wire.go`, and `backend/internal/handler/wire.go`.
- `backend/internal/handler/handler.go` defines the top-level `Handlers` container and nested `AdminHandlers` container used by route registration.
- Adding a new handler/service/repository usually means updating both the constructor and the relevant provider set.

Ent is used similarly:

- Hand-written schema: `backend/ent/schema/`
- Generated ORM code: `backend/ent/`
- Database migrations: `backend/migrations/*.sql`

Do not rely on Ent auto-migrate here; schema changes require raw SQL migrations plus regenerated Ent code.

### Settings KV is the runtime configuration spine

Most admin-editable runtime behavior goes through the Settings KV stack rather than dedicated config files.

The cross-file chain is:

- setting keys/constants in `backend/internal/service/domain_constants.go`
- persistence in `backend/internal/repository/setting_repo.go`
- higher-level assembly/caching in `backend/internal/service/setting_service.go`
- public exposure through `GET /api/v1/settings/public`
- frontend caching/consumption in `frontend/src/stores/app.ts`

If you add a new public setting, expect to touch backend constants/service/DTOs, the admin settings surface, and the frontend app-store cache path.

### Frontend architecture

The frontend is a Vue 3 SPA embedded into the backend for production.

The main cross-file control points are:

- `frontend/src/router/index.ts` — route truth source, auth/admin guards, title metadata
- `frontend/src/api/client.ts` — Axios instance with auth token injection, refresh flow, locale header, timezone param, and response-envelope unwrapping
- `frontend/src/stores/app.ts` — global toasts, loading state, cached public settings, version info
- `frontend/src/stores/auth.ts` — persisted auth/session state
- `frontend/src/i18n/locales/{zh,en}.ts` — i18n truth sources; keep both in sync

A subtle but important pattern: both dev and production inject public settings into the initial HTML.

- Production does it through the backend embedded-frontend middleware in `backend/internal/server/router.go` / `backend/internal/web`.
- Dev mode mirrors that in `frontend/vite.config.ts` by fetching `/api/v1/settings/public` before serving `index.html`.

This keeps auth/setup pages from rendering with stale defaults before the first async settings fetch.

### Cross-repo integration boundaries

This repository has explicit local integration points with sibling checkouts:

- `AIClient2API`
- `InvokeAI`
- `new-api`

Those relationships are documented in `docs/dev/RELATED_PROJECTS.md`. When a task changes an integration contract, update both Sub2API docs and the sibling repository's own integration/rule docs.

## Project-specific rules and pitfalls

- Append every verified local change to `docs/dev/CHANGELOG_CUSTOM.md`.
- Keep `frontend/pnpm-lock.yaml` committed; never switch this repo to npm.
- Use `127.0.0.1`, not `localhost`, for local PostgreSQL/Redis on Windows.
- Viper checks `E:\app\data\config.yaml` before local config; a stale file there can silently override `backend/config.yaml`.
- New i18n keys must be added to both `frontend/src/i18n/locales/zh.ts` and `frontend/src/i18n/locales/en.ts`.
- After deep exploration or a bug fix that reveals module invariants, update the relevant `docs/dev/codebase/*.md` file.
- For Nginx reverse proxies in front of Codex/OpenAI-compatible flows, set `underscores_in_headers on;` or sticky-session headers such as `session_id` can be dropped.
