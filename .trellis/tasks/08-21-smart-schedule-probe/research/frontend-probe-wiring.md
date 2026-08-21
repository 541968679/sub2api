# Frontend wiring: smart-schedule probe (`probing`)

Date: 2026-08-21. Backend field names in `research/probe-api-contract.md` win.

## Wired

| Surface | How |
| --- | --- |
| Live states | `paused` / `cooling` / `probing` / `selectable` / `resumed` in `PAIR_ADMISSION_LIVE_STATES` (switcher order: 暂停 / 冷却 / 考察 / 调度 / 豁免期) |
| Display + filter | `PoolAdmissionState` + `POOL_ADMISSION_FILTER_STATES` include `probing` |
| Hydration | Member `probing: true`, or `admission` / `state` === `probing`. Missing mark = not probing (no backfill). Expired `cooldown_until` does **not** invent probing |
| Probe cap | Prefer GET/POST `probe_cap`. Keep `probing_cap` / `in_flight_cap` / `pair_probe_cap` as read aliases only. Else `min(N, member cap)` or `N` |
| Occupancy badge | Probe rows use that cap (never 999) |
| Switch POST | `POST /admin/accounts/:id/smart-schedule-resume` `{ user_id, state: "probing" }` via `resumeSmartSchedule`. Backend `ParsePairAdmissionState` / `SetPairAdmission` accept `probing` (no 400) |
| Local apply | Clearing pause does **not** write probing. Next state is whatever the admin picked. Selectable → probing is allowed |
| Pair events | Labels for `probe_enter` / `probe_graduate` (plus `probing` / `enter_probing` aliases) |
| i18n | zh+en: `admissionProbing`, `admissionProbingHint`, `switchSuccessProbing`, `pairOccupancyProbingHint`, `pairEventProbeEnter`, `pairEventProbeGraduate`, pause/exemption copy |

## Landed (backend sibling)

| Item | Notes |
| --- | --- |
| GET `probing` on pool members | Always present (`false` when not probing). UI hydrates from it |
| GET/POST `probe_cap` | Present only when `state=probing`. In-flight cap actually enforced |
| `ParsePairAdmissionState("probing")` | Accepted. Invalid still `SMART_SCHEDULE_ADMISSION_INVALID`. Omitted `state` stays `resumed` |
| Cooldown expiry → probing | Backend `expirePairCooldown` zeros windows, writes probe mark, no `u:`/`w:`. Frontend does **not** invent probing from an expired `cooldown_until` |
| Event persist `probe_enter` / `probe_graduate` | Detail list returns them; dialog labels them |
| `research/probe-api-contract.md` | Authoritative field names: `probing` + `probe_cap` |

## Intentionally not wired

- No “解除暂停默认考察” button or implicit unpause destination
- Omitted `state` on resume still means 豁免期 (`resumed`), not `probing`
- Account-quality last-N / hard-close APIs and cells
- Client protocol / stream / endpoint changes
