# Sub2API Agent Instructions

This file is the entry point for AI agents (Codex, Copilot, Claude Code, etc.)
working on this repository. When this file and `CLAUDE.md` differ, prefer the
more specific rule here or in the referenced architecture/codebase documents.

## Repository Rule Boundaries

- When working in this checkout (`E:\cursor project\api2sub`), follow this file
  as the Sub2API repository rule entry point.
- When work belongs to a sibling checkout, switch to that repository root and
  follow that repository's own `AGENTS.md`. This file records integration
  boundaries but does not replace sibling-project engineering rules.
- Do not edit sibling project source files from a Sub2API task unless the
  current task explicitly requires cross-repository changes.
- Cross-repository contract changes must update both sides: Sub2API integration
  documentation and the affected sibling project's own documentation.
- The cross-project relationship index is `docs/dev/RELATED_PROJECTS.md`.

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

## GitHub CLI Credentials

- The expected local GitHub CLI account is `541968679`. Check it with
  `gh auth status` before using `gh` for issue/PR/search/API work.
- `gh` credentials should live in the Windows keyring. A healthy status looks
  like `Logged in to github.com account 541968679 (keyring)`.
- If `gh` is not logged in, first try the browser/device flow:

```powershell
gh auth login --web --hostname github.com --git-protocol https
```

- If the device page does not open, open it explicitly and enter the one-time
  code printed by `gh`:

```powershell
cmd /c start "" "https://github.com/login/device"
```

- If the device flow succeeds in the browser but `gh auth status` is still not
  logged in, recover from the local PAT only by piping it directly into `gh`.
  Do not print, paste into chat, commit, or log the token. One historical local
  source is the sibling checkout `E:\cursor project\AIClient2API\.git\config`;
  verify the file exists before using it:

```powershell
$cfg = 'E:\cursor project\AIClient2API\.git\config'
$text = Get-Content -LiteralPath $cfg -Raw
$token = [regex]::Match($text, 'github_pat_[A-Za-z0-9_]{22,}|gh[pousr]_[A-Za-z0-9]{36,}').Value
if (-not $token) { throw "No GitHub token found in $cfg" }
$token | gh auth login --with-token --hostname github.com
Remove-Variable token
```

- Never add a PAT or unmasked `ghp_...` / `github_pat_...` value to
  `AGENTS.md`, docs, commits, shell output, or chat responses. If a token is
  exposed, rotate it before continuing GitHub work.

## Start Here

Before changing unfamiliar code, read these in order:

1. `docs/dev/ARCHITECTURE.md` - top-level architecture, request flow, Wire,
   Settings KV, migrations, common task templates, known pitfalls.
2. `docs/dev/codebase/README.md` - module documentation index.
3. The relevant `docs/dev/codebase/{module}.md` file if it exists.
4. `docs/dev/RELATED_PROJECTS.md` when the task touches AIClient2API,
   InvokeAI, new-api, sidecars, ports, or cross-repository contracts.
5. `CLAUDE.md` only when you need the fuller historical context.

If deep exploration traces a flow across three or more files, update or create
the matching `docs/dev/codebase/*.md` entry using this shape:
data model -> key files -> core flow -> important mechanisms -> known pitfalls.

Update `docs/dev/ARCHITECTURE.md` only for top-level modules, cross-cutting
conventions, reusable task templates, or environment/build pitfalls.

## Task Signal Reading Chain

Pick the first document from the task signal. Do not treat every task as a
code-exploration walk.

| Task signal | Read first |
|-------------|------------|
| Unfamiliar code / change a module | Existing Start Here chain: `ARCHITECTURE.md` → `codebase/README.md` → `codebase/{module}.md` |
| 生产 / 线上 / 报错 / 排查 / 日志 / incident | `docs/dev/PRODUCTION.md` — pull logs before reading or changing code |
| Deploy / release process | `docs/dev/DEPLOYMENT.md` — not the same as an incident |
| 上游 / upstream / 同步 / sync / worktree / 合并副本 | `.cursor/rules/upstream-sync.mdc` then this file's **Upstream sync** section — assess before changing product code |

## Upstream sync (MUST)

When the task is pulling updates from upstream `Wei-Shaw/sub2api` (or porting a prior sync branch):

- MUST write an assessment table and wait until it has been presented (and confirmed, for a new window) before changing sync-related product code.
- MUST classify each theme into a lane; do not absorb the window wholesale.

| Lane | Take | Do not |
|------|------|--------|
| **A hotfix** | Protocol, failure class, identity, failover — no display-billing change | Touch stored billing / display transform |
| **B 3-way** | Same-file overlay on gateway / scheduler / usage after reading fork-local use | Whole-file or whole-hunk replace from upstream / old sync branch |
| **C defer/reject** | Living catalog (Grok wholesale, channel-monitor v2, Passkey, plaza, deploy-path swap, bill-by-upstream-model) | Re-litigate the whole catalog every week |

### Assessment table columns (minimum)

upstream commit/feature point; concrete behavior change; affected backend/frontend modules; frontend-visible; how to test; relation to fork-local secondary development; expected impact on fork-local features; risk; handling (`A` / `B` / `C` or 采纳/适配/精细/延后/拒绝/已有). The fork-local impact column is mandatory and must call out applicable areas: billing/display-token accounting, curated/default model lists, Claude-GPT bridge, OpenAI image generation, default-model fallback, account scheduling/failover/pair, ops logging, public/admin settings, migrations, frontend i18n/routes.

### Workspace

- Occupied checkout `E:\cursor project\api2sub` stays on `main`. Do not `checkout` it away from `main` for sync work.
- Sync **product** code is written only in an isolation worktree (new branch; never reuse an old catchup branch as the continue-stacking point).
- Rule/doc edits (`AGENTS.md`, `.cursor/rules`) may be made on that occupied `main` checkout without switching branches.
- Old sync worktrees/branches (including `E:\cursor project\api2sub-upstream-catchup-20260814` and `sync/upstream-catchup-*`) are patch sources only. Do not delete them. Do not `merge` / `rebase` them into `main` or into a new replica.

### Fine-grained port (hard)

- Never `git merge upstream/main` as the delivery form.
- Never replace a hot-path file or hunk wholesale from upstream or an old sync branch. Read the fork-local use of each shared file first, then overlay behavior.
- If a functional conflict cannot be resolved without breaking fork invariants (display prices not from `cost/tokens`; display must not change stored billing / `actual_cost`; real `cache_read` + `display_cache_token_max_mult`; Claude-GPT bridge; Batch Image; scheduler/pair; GHCR deploy policy), **stop and ask Brandon**. Do not pick a side or delete fork-local behavior.
- New SQL is additive only. Number from **current `main` max+1**. Do not edit historical migrations. Do not reuse numbers already on `main`.
- Do not set this repo's `VERSION` to an upstream version number. Record waterline only in `docs/dev/UPSTREAM_BASE.json` / `UPSTREAM_SYNC.md` (`git add -f`).
- Do not merge a sync branch into real `main`, `git push`, or deploy without explicit Brandon authorization **for that action**.

Cursor short form: `.cursor/rules/upstream-sync.mdc`. The `.cursor/` tree is
gitignored; `git add -f` that rule file when committing.

## Production incidents (MUST)

If the task includes 生产 / 线上 / 报错 / 排查 / 日志 / incident:

- MUST read `docs/dev/PRODUCTION.md` first, SSH-pull logs, then look at code.
- MUST NOT search business code or change implementations first.
- Wrong: Grep `gateway_handler.go` first. Right: SSH `tail` / `docker inspect` health first.

| Item | Value |
|------|-------|
| Host | `root@172.245.247.80` |
| SSH key | `%USERPROFILE%\.ssh\id_ed25519_sub2api` / `~/.ssh/id_ed25519_sub2api` |
| Compose | `/opt/sub2api` |
| Deploy log | `/opt/sub2api/deploy.log` |
| Image | `ghcr.io/541968679/sub2api:latest` |

```powershell
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose ps"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "docker inspect sub2api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}}'"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose logs --tail=120 aiclient2api"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "tail -n 120 /opt/sub2api/deploy.log"
```

The same §1.1 block also has `docker compose logs --tail=120 invokeai`. Full
playbook: `docs/dev/PRODUCTION.md`. Release flow: `docs/dev/DEPLOYMENT.md`.

- No push/deploy without explicit user permission for this turn.
- Production main service: GHCR pull only. No production-host `docker build` /
  `sub2api-custom:*`.
- Never redeploy `v0.1.232` or `v0.1.233`.
- Gateway 524 / sync-timeout / failover work must not require clients to
  change protocol, `stream`, endpoint, or request body. Keep inbound sync
  `/v1/chat/completions` (`stream:false`) and fix it in the gateway.

## Local Development Environment

### Port Rules (STRICT — never use low ports)

| Service | Local Port | Config Location |
|---------|-----------|-----------------|
| sub2api backend | **18081** | `backend/config.yaml` → `server.port` |
| sub2api frontend | **15174** | `frontend/.env.development.local` → `VITE_DEV_PORT` |
| AIClient2API API | 3000 | `E:\cursor project\AIClient2API` project config |
| AIClient2API Master | 3100 | Same project |
| new-api API | **13200** | Optional sibling project `E:\cursor project\new-api`, via `scripts/dev-stack.ps1 -IncludeNewAPI` |
| InvokeAI backend | **9090** | Sibling project `E:\cursor project\InvokeAI`, via its own `scripts\dev-stack.ps1` |
| InvokeAI frontend | **15175** | Sibling project `E:\cursor project\InvokeAI`, via its own `scripts\dev-stack.ps1` |
| PostgreSQL | 5432 | **Host** Windows service `postgresql-x64-16` (shared-infra; logical DB `sub2api`) — not Docker for daily dev |
| Redis | 6379 | **Host** Windows service `Redis` (shared-infra) — not Docker for daily dev |

**Forbidden local ports**: 8080, 8081, 5173, 5174, 5175 — these conflict with
Docker containers and other services. Production ports are managed by server
Docker Compose and are completely independent of local config.

### Starting Local Services

All normal local start/restart/stop actions must go through the repo script:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 restart
```

or:

```bat
scripts\dev-stack.cmd restart
```

Use `-SkipAIClient` when you only need sub2api.
Use `-IncludeNewAPI` when you also need the sibling `new-api` backend; the
script starts it through a generated Docker Compose file and maps it to
`http://127.0.0.1:13200` by default. Override with `-NewAPIPath` or
`-NewAPIPort` only for targeted local debugging.
Do not launch `air.exe`, `pnpm dev`, or `npm start` directly for normal local work.

### Related Local Subprojects

Details live in `docs/dev/RELATED_PROJECTS.md`. The table below is only a quick
index; each sibling repository's own `AGENTS.md` is authoritative for work done
inside that checkout.

| Project | Local Path | Local Endpoint/Ports | Sub2API Relationship |
|---------|------------|----------------------|----------------------|
| AIClient2API | `E:\cursor project\AIClient2API` | API/Web UI `3000`, Master `3100` | Optional client-proxy sidecar; production compose has `aiclient2api` |
| new-api | `E:\cursor project\new-api` | Sub2API-side local mapping `127.0.0.1:13200` | Optional local integration only; production gateway/account wiring not implemented |
| InvokeAI | `E:\cursor project\InvokeAI` | Backend `9090`, frontend `15175` | External API image UI sidecar; CPU/external-provider only |

Do not commit generated cross-project runtime files such as
`tmp/dev-stack/new-api.compose.yml`.

### Hot Reload

- **Frontend**: Vite HMR built-in, saves auto-refresh browser.
- **Backend**: `air` watches file changes, auto-compiles and restarts. No manual
  kill/restart needed. Config in `backend/.air.toml`.

### Known Config Pitfall

Viper config loading searches `/app/data` (resolves to `E:\app\data\` on
Windows) BEFORE the current directory. If `E:\app\data\config.yaml` exists, it
will override `backend/config.yaml`. If the backend unexpectedly binds to port
8080, check and remove/rename that file.

## Key Linked Files

### Core Documentation

- `CLAUDE.md` - original project context and operating rules.
- `DEV_GUIDE.md` - local development setup and common environment issues.
- `docs/dev/ARCHITECTURE.md` - required architecture entry point.
- `docs/dev/CHANGELOG_CUSTOM.md` - custom-change log; append every verified
  local change here.
- `docs/dev/SECONDARY_DEV.md` - secondary development guide.
- `docs/dev/PRODUCTION.md` - production incident / log playbook (host, SSH,
  pull-logs-first).
- `docs/dev/DEPLOYMENT.md` - deployment and operations guide.
- `docs/dev/SECURITY_OPERATIONS.md` - credential rotation and security
  operations guide.
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

### Billing Display Invariants

These rules are core product behavior. Do not change them casually, and add or
update regression tests whenever touching usage display pricing, user-group
display rates, cache token rewriting, or admin/user usage previews.

- User-facing model unit prices must come from configured display prices or the
  normal pricing chain (`User -> Channel -> Global -> LiteLLM -> Fallback`).
  Frontend code must not derive displayed model prices from `cost / tokens`.
- Display transforms may change user-visible tokens and component costs, but
  must never change stored billing, quota deduction, or `actual_cost`.
- Display `cache_read` may amplify under model display prices (L1) and/or group
  display rate (L2), but must stay within
  `display_cache_read <= billing_real_cache_read * M_eff`
  (`display_cache_token_max_mult`, default 1.3). Cap is relative to **billing-real**
  DB tokens. Cost/token mass that cannot sit on cache under that cap folds into
  output (alpha) then input - do not invent unbounded cache tokens.
- When cache-read display price is configured, displayed `cache_read_cost` must
  remain explainable as
  `display_cache_read_tokens * display_cache_read_price`.
- The displayed bill must stay explainable from displayed tokens, displayed unit
  prices, and the displayed rate multiplier:
  `display_total_cost * display_rate_multiplier ~= actual_cost`, allowing only
  small integer-token rounding tolerance.
- Admin list `display_fields`, admin `user-view`, and user `/usage` must share the
  same L1+L2 transform and unit-price resolution path (including rate-only rows
  with no per-model display override).

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
- Production Sub2API main-service deployments must use the GitHub Actions-built
  GHCR image `ghcr.io/541968679/sub2api:latest` or an explicitly approved tag.
  The historical production-host `docker build` / `sub2api-custom:latest` path
  is legacy and must not be used for future main-service deploys.
- `tools/check_pnpm_audit_exceptions.py` - pnpm audit exception checker.

## Development Commands

Backend:

```bash
go generate ./ent
go generate ./cmd/server
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o bin/server ./cmd/server
```

For normal local service startup, use `scripts/dev-stack.ps1` or
`scripts/dev-stack.cmd`. Use raw `go run` or `air.exe` only for targeted
debugging.

Frontend:

```bash
pnpm install --frozen-lockfile
pnpm build
pnpm run lint:check
pnpm run typecheck
pnpm run test
```

For normal local service startup, use `scripts/dev-stack.ps1` or
`scripts/dev-stack.cmd`. Use raw `pnpm dev` only for targeted debugging.

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
- Before every upstream-sync batch, follow **Upstream sync (MUST)** above
  (assessment table, A/B/C lanes, isolation worktree, fine-grained overlay,
  ask Brandon on unresolved conflicts). Do not merge `upstream/main` wholesale
  or replace hot-path files. The short Cursor form is
  `.cursor/rules/upstream-sync.mdc`.
- Append every verified change to `docs/dev/CHANGELOG_CUSTOM.md` with what
  changed, why, and affected files.
- Sub2API repository changes are logged in `docs/dev/CHANGELOG_CUSTOM.md`.
  Sibling-project internal changes are logged in that sibling project's own
  changelog when it has one. Cross-repository contract changes must be recorded
  on both sides.
- Commit verified local changes promptly. Do not push or deploy without explicit
  user permission for that specific push/deploy.
- `docs/dev` may be ignored by Git; use `git add -f` for required doc updates.
- Use `pnpm` only. Do not use `npm`, do not create `package-lock.json`, and
  always keep `frontend/pnpm-lock.yaml` committed.
- Use `scripts/dev-stack.ps1` or `scripts/dev-stack.cmd` for any local service
  start, restart, or stop action. Direct `air.exe`, `pnpm dev`, and `npm start`
  are reserved for debugging the script itself.
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
- For production Sub2API main-service deploys, verify that GitHub Actions has
  published the intended GHCR image first, then deploy by pulling that image with
  Docker Compose. Do not run server-side `docker build` or rely on
  `sub2api-custom:*`; if `deploy/update.sh` still does this for the main
  service, update the deployment tooling before deploying.

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
- Viper config loading searches `E:\app\data\` before `./` on Windows. A stale
  `config.yaml` there will silently override local config. If backend binds to
  wrong port, check that path first.
- Local backend uses port **18081** (configured in `backend/config.yaml`). Never
  use `8080` or `8081` — they conflict with Docker containers.
