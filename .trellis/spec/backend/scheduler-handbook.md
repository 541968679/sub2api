# Scheduler Handbook (pointer)

## Scenario: change account selection

### 1. Scope / Trigger

- Trigger: changing account selection, sticky session, failover exclude, scheduler snapshot/outbox, smart-schedule admission, OAuth fleet soft 429, `fallback_only`, or OpenAI scheduler scoring.
- Canonical: `docs/dev/codebase/scheduler.md` (map + forbidden + file pointers; not an algorithm dump).
- Do not implement from `account.md` / `gateway.md` / this file alone.

### 2. Adjacent specs

Field-level contracts stay in these files. Do not copy them into the handbook.

- `account-user-schedule.md`
- `oauth-fleet-soft-429.md`
- `account-quality-hard-close.md`
- `account-quality-snapshots.md`
- `ops-schedule-error-caliber.md`

Management PnL: `schedule-pnl.md` only.

### 3. Forbidden

- Do not fold user policy into `IsSchedulable()`.
- Do not whole-file replace the upstream scheduler (or overlay/fork hot paths) during a sync window.
- Do not paste scoring formulas, Redis key layouts, or numeric defaults into docs.

### 4. Next step

Read `docs/dev/codebase/scheduler.md` for the file map and forbidden list. Then open the source symbol and the adjacent spec for the field you are editing.
