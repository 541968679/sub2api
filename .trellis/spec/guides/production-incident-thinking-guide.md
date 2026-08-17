# Production Incident Thinking Guide

> **Purpose**: Stop the “search the repo, then guess the host” loop. Production entry is already known; logs come before code.

---

## When to Use

- [ ] The user said 生产 / 线上 / 报错 / 排查 / 日志 / incident
- [ ] A live user or admin report needs root-cause, not a feature design
- [ ] Deploy/health looks wrong and you are about to open Go files

→ Read `docs/dev/PRODUCTION.md` first. Compact MUST facts also live in `AGENTS.md` and `.cursor/rules/ops-incident.mdc`.

---

## Checklist (in order)

1. **Do not Grep `docs/dev/`**. That tree is gitignored; search will look empty. Discovery is always-apply, not search.
2. **Read the playbook**: `docs/dev/PRODUCTION.md`. Deploy/preflight/history stay in `docs/dev/DEPLOYMENT.md`.
3. **Pull logs before code**: `docker compose ps`, `docker inspect` health, `docker compose logs`, `tail /opt/sub2api/deploy.log`.
4. **If the product error tables matter**, query `/api/v1/admin/ops/request-errors`, `/upstream-errors`, `/errors` (see `docs/dev/codebase/ops.md`).
5. **Only then** open gateway/billing/scheduler code.
6. **Do not push/deploy** unless the user approved it in this turn. Production is GHCR pull only. Never redeploy `v0.1.232` / `v0.1.233`.
7. **Do not write the current running tag** into always-apply rules. Current belongs in the `DEPLOYMENT.md` history table.

---

## Common Mistakes

| Symptom | Cause | Fix |
|---------|--------|-----|
| Agent “finds” production by browsing chat history | `docs/dev` is search-invisible | Use `AGENTS.md` / `ops-incident.mdc` compact table |
| Agent explains a prod error from `gateway_handler.go` only | Skipped logs | SSH/log commands first |
| Always-apply lists `v0.1.234` as current | Status leaked into rules | Point at `DEPLOYMENT.md` history; keep only banned tags in rules |
| Commit “succeeds” but `PRODUCTION.md` is missing | `docs/*` and `.cursor/` are gitignored | `git add -f` those paths; do not change `.gitignore` for this |

---

## Wrong vs Correct

#### Wrong
Open `backend/internal/handler/gateway_handler.go`, then ask where production lives.

#### Correct
Read `PRODUCTION.md` → pull compose/inspect/`deploy.log` → then read the handler that the logs actually implicate.
