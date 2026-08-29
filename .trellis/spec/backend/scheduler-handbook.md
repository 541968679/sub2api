# Scheduler Handbook (pointer)

## Scenario: change account selection

### 1. Scope / Trigger

- Trigger: changing account selection, sticky session, failover exclude, scheduler snapshot/outbox, smart-schedule admission, OAuth fleet soft 429, `fallback_only`, or OpenAI scheduler scoring.
- Canonical: `docs/dev/codebase/scheduler.md`.
- Do not implement from `account.md` / `gateway.md` / this file alone.

### 2. Adjacent specs

Field-level contracts stay here (do not copy handbook chapters):

- `account-user-schedule.md`
- `oauth-fleet-soft-429.md`
- `account-quality-hard-close.md`
- `account-quality-snapshots.md`
- `ops-schedule-error-caliber.md`

Management PnL: `schedule-pnl.md` only.

### 3. Forbidden

- Do not fold user policy into `IsSchedulable()`.
- Do not whole-file replace the upstream scheduler (or overlay/fork hot paths) during a sync window.

### 4. Next step

Read `docs/dev/codebase/scheduler.md` §5 (hard-gate order), §9 (files to change for a new exclude layer), and §10 (forbidden patterns). Then open the adjacent spec for the field you are editing.
