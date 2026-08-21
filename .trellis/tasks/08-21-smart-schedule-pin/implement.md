# Implement: smart-schedule `pinned`

## Order

1. Write `research/pin-api-contract.md` (signatures + good/bad).
2. Cache + interfaces: `IsPinned` / `MarkPinned` / `ClearPinned` / `IsPinnedBatch`; Redis HASH; `StartCooldown` no-op while pinned; `expirePairCooldown` never pins.
3. `SetPairAdmission` + `ParsePairAdmissionState`; GET hydrate `pinned`.
4. Hot path: `admitsScheduleUser`, `ObservePairCompletion`, `evaluateSmartSchedulePairQuality`.
5. Frontend: types, composable, switcher, i18n, editor local state.
6. Tests listed below.
7. Update `.trellis/spec/backend/account-user-schedule.md` + `docs/dev/CHANGELOG_CUSTOM.md`.

## Validation

```powershell
go test -tags=unit ./internal/service -count=1 -run "Pinned|ParsePairAdmission|SetPairAdmission|AdmitsScheduleUser|ObservePairCompletion|Expiry"
go test -tags=unit ./internal/repository -count=1 -run "PairQualityCache_|Pinned"
pnpm --dir frontend exec vitest run src/composables/__tests__/smartSchedulePoolAdmission.spec.ts src/composables/__tests__/useUserSmartScheduleEditor.spec.ts src/views/admin/__tests__/UserSmartScheduleView.spec.ts
```

## Tests required

- Enter `state=pinned`: cooldown gone, probe gone, no `u:`/`w:`, `IsPinned`, GET `pinned: true`.
- Omit `state` / `""` → `resumed`, not `pinned`.
- Cooldown expiry → `probing`, `IsPinned==false`.
- N successes (or breached windows) while pinned: ingest happens, `StartCooldown` does not.
- Leave to `selectable`, then N breaches → cools again.
- Pause does not write pin; omit-state after pause is `resumed`, not `pinned`.

## Rollback points

- Redis pin HASH is additive; readers treat miss as not pinned.
- Do not edit historical migrations.
- Do not commit / push / deploy in this task.

## Risky files

- `backend/internal/service/account_user_schedule.go` — pin check **before** cooldown reject.
- `backend/internal/repository/user_smart_schedule_cache.go` — no TTL; do not `Expire` the pin HASH.
- `frontend/src/composables/useUserSmartScheduleEditor.ts` — `applyLocalAdmission` must **not** write local `u:`/`w:` for `pinned`.
