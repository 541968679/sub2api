# Display Token Pricing (Usage + Downstream)

> L1 model display prices + L2 group display rate; B1 cache_read amplify capped by M.

## Scope / Trigger

Touch this when changing `ApplyDisplayTransform`, `ApplyUserDisplayRate*`,
admin `display_fields` / `user-view`, user `/usage`, or downstream
`computeSeparatedDisplayUsage`.

## Contracts

```text
DB real → L1 (Display*Price + AllocateDisplayTokens M/α)
       → L2 (scale = real_rate/display_rate; B1 cache ≤ billing_real × M)
       → absolute cap (write path + applied=true replay only)
       → user /usage | admin display_fields | user-view | downstream display mode
```

- Cap vs **billing-real** cache tokens (pre-L1).
- L1+L2 read path does not rewrite `actual_cost`.
- After L1+L2, optional joint input+cache cap (`display_context_token_max`) +
  independent output cap (`display_output_token_max`). Code default 0 = off.
- Joint cap jitters the **sum only** (`hash(request_id+"|joint")`, 92%–100%).
  If `S > C`: `in' = round(in*C/S)`, `cache' = C-in'` (clamp to pre-cap).
  Discard leftover; no residual fold. Output uses lane `|output`.
- Write path (token mode): when a cap binds, replace `actual_cost` with
  `display_total' × display_rate` (lower than uncapped). Snapshot
  `display_token_cap_applied` + used configured caps. Billing-real tokens stay.
- Read path: `applied=false` → L1+L2 only (history freeze). `applied=true` →
  replay using row used caps + `request_id`.
- `display_total * display_rate ≈ actual_cost` (new capped rows use the new lower bill).
- Admin/user/downstream must share one user-visible builder + unit-price resolution.
- Rate-only rows (no `HasDisplayOverride`) still emit admin `display_fields` when rates differ.

## Wrong vs Correct

**Wrong**: Admin list L1-only / require `HasDisplayOverride`; L2 never touches cache_read.

**Correct**: Shared L1+L2; B1 amplifies cache_read up to `real × M`.
