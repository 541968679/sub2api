# Sub2API Related Projects

## Purpose

This document records integration boundaries between Sub2API and sibling
checkouts on this workstation. It is the Sub2API-side index for cross-repository
work; it does not replace the sibling repositories' own `AGENTS.md` files.

When a task belongs inside a sibling checkout, switch to that repository root and
follow that repository's local rules. When a task changes a contract between
Sub2API and a sibling project, update both sides of the documentation.

## Project Map

| Project | Local Path | Rule Entry | Integration Doc | Sub2API Relationship | Current Scope |
|---------|------------|------------|-----------------|----------------------|---------------|
| AIClient2API | `E:\cursor project\AIClient2API` | `E:\cursor project\AIClient2API\AGENTS.md` | `E:\cursor project\AIClient2API\docs\SUB2API_INTEGRATION.md` | Optional client-proxy sidecar for Kiro, Gemini CLI, Antigravity, Qwen, and custom OpenAI/Claude routes | Local API/Web UI on `3000`, Master on `3100`; production compose includes `aiclient2api` |
| InvokeAI | `E:\cursor project\InvokeAI` | `E:\cursor project\InvokeAI\AGENTS.md` | `E:\cursor project\InvokeAI\docs\SUB2API_INTEGRATION.md` | External API image UI sidecar managed by Sub2API | Backend `9090`, frontend `15175`; only external OpenAI/Sub2API/Gemini-style providers, no local GPU/model inference |
| new-api | `E:\cursor project\new-api` | `E:\cursor project\new-api\AGENTS.md` | `E:\cursor project\new-api\docs\SUB2API_INTEGRATION.md` | Optional sibling gateway for local integration experiments | Sub2API local script maps it to `127.0.0.1:13200`; production Sub2API gateway/account wiring is not implemented |

## Port Matrix

| Service | Local Port | Owner | Notes |
|---------|------------|-------|-------|
| Sub2API backend | `18081` | `api2sub` | Configured in `backend/config.yaml` as `server.port` |
| Sub2API frontend | `15174` | `api2sub` | Configured by `frontend/.env.development.local` / `VITE_DEV_PORT` |
| AIClient2API API/Web UI | `3000` | `AIClient2API` | Keep this port for local and production-side assumptions |
| AIClient2API Master | `3100` | `AIClient2API` | Managed by AIClient2API runtime |
| new-api API for Sub2API testing | `13200` | `api2sub` generated compose | Use `scripts/dev-stack.ps1 -IncludeNewAPI`; do not edit `new-api/docker-compose.dev.yml` just to avoid local port conflicts |
| InvokeAI backend | `9090` | `InvokeAI` | Local APIs; root redirects to the frontend during local development |
| InvokeAI frontend | `15175` | `InvokeAI` | Local UI; Vite proxies API/websocket traffic to `9090` |
| PostgreSQL | `5432` | `api2sub` dev stack | Docker container `sub2api-pg-dev` |
| Redis | `6379` | `api2sub` dev stack | Docker container `sub2api-redis-dev` |

Forbidden local ports for Sub2API work: `8080`, `8081`, `5173`, `5174`, and
`5175`.

## Startup Boundaries

- Start, restart, stop, and status for Sub2API through
  `scripts/dev-stack.ps1` or `scripts/dev-stack.cmd`.
- Start InvokeAI through its own `scripts\dev-stack.ps1` or
  `scripts\dev-stack.cmd`; do not start `invokeai-web`, `pnpm dev`, or
  `make frontend-dev` directly for normal local development.
- Start new-api for Sub2API local integration through
  `scripts/dev-stack.ps1 -IncludeNewAPI`. The generated compose file lives under
  `tmp/dev-stack/` and must not be committed.
- Keep AIClient2API on its configured `3000`/`3100` ports unless the current task
  explicitly changes the integration contract and updates both repositories'
  documentation.

## Change Logging Rules

- Sub2API-only changes go in `docs/dev/CHANGELOG_CUSTOM.md`.
- Sibling-project internal changes go in that sibling project's own changelog
  when one exists.
- Cross-repository contract changes must be recorded in both the Sub2API docs
  and the affected sibling project's integration docs.
- Do not rewrite historical changelog entries only to move them between
  repositories; apply this boundary to new work going forward.

## Production Boundaries

- Production Sub2API main-service deployments must use the approved GHCR image
  flow documented in `docs/dev/DEPLOYMENT.md` and
  `docs/dev/PRODUCTION_CUSTOM_IMAGE_DEPLOY.md`.
- `AIClient2API` and `InvokeAI` are sidecars in the Sub2API production compose
  flow. Sibling source trees are not vendored into this repository.
- `new-api` production deployment and account/gateway wiring from Sub2API are
  not implemented yet.
