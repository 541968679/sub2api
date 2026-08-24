# OAuth fleet soft 429

## Scenario: blip 429 is request-exclude, not global unschedule

### 1. Scope / Trigger

- Trigger: OAuth / setup-token upstream HTTP 429 when the policy applies to that account. Cross-layer: Settings KV + Admin GET/PUT + Redis scheduler filter + account `extra` overlay.
- Settings KV `oauth_fleet_soft_429_settings`. Factory `DefaultOAuthFleetSoft429Settings().Enabled=false`. Empty / missing / bad JSON = OFF (do **not** copy 529 overload cooldown empty-KV=on).
- Account overlay is `accounts.extra.oauth_fleet_soft_429` only (`true` / `false` / unset). Never credentials. Never `IsPoolMode()`.
- Soft exclude Redis key `oauth-soft-429:{accountID}` is a scheduler filter only. Do not fold it into `IsSchedulable()`, pair/quality admission, or ops/quality caliber.

### 2. Signatures

- `oauthFleetSoft429Applies(account, settings)` — extra false → off; extra true → canary on; else global enabled + scope / setup-token.
- `classifyOAuth429(...)` → soft | hard | not_applicable. Built-in hard (Anthropic window, Codex 100%, quota death) beats the admin code table.
- `RateLimitService.HandleUpstreamError` classifies **before** `tryTempUnschedulable`.
- `RateLimitService.TryHandleOAuthFleetSoft429` — Gemini / Antigravity paths that skip `HandleUpstreamError` on 429.
- `MergeOAuthFleetSoft429Exclusions` / `mergeOAuthFleetSoft429ExcludedIDs`
- `oauthFleetSoft429HasHardAffinity(previousResponseID, stickyAccountID)` — skip layer-2 only when an existing sticky **binding** or `previous_response_id` is present. A generated `sessionHash` is not affinity.
- Admin (not public):
  - `GET /api/v1/admin/settings/oauth-fleet-soft-429`
  - `PUT /api/v1/admin/settings/oauth-fleet-soft-429`
- DTO / KV JSON: `enabled`, `ttl_seconds` (5–300, default 20), `long_reset_policy` (`soft`|`hard`|`threshold`), `long_reset_threshold_seconds` (5–86400, default 60), `scope` (`all_oauth`|`opt_in`), `platforms`, `include_setup_token`, `soft_status_codes`, `soft_body_codes`, `hard_body_codes`.

### 3. Contracts

| Condition | Result |
|---|---|
| Soft + applies | no `SetRateLimited` / `SetTempUnschedulable`; Redis SET TTL=`ttl_seconds`; `failedAccountIDs` still gets the id; `RetryableOnSameAccount` stays false |
| Hard window / quota death | persist as today; soft code table cannot soften |
| Extra true + global off / empty KV | canary soft path |
| Extra false or unset + global off | current `handle429` |
| New request, `sessionHash` set, no sticky binding | layer-2 exclude applies |
| Existing sticky binding or `previous_response` | layer-2 skipped; pin stays |
| API Key `pool_mode` | unchanged early-return / same-account retry |
| `GET /settings/public` | must not contain this policy |
| Redis GET/SET fail | fail-open (do not block scheduling / persist) |
| PUT invalid ttl / policy / platforms | 400; KV unchanged |

### 4. Validation & Error Matrix

| Condition | Error |
|---|---|
| `ttl_seconds` outside 5–300 | `ttl_seconds must be between 5-300` |
| `long_reset_policy` not `soft`/`hard`/`threshold` | invalid policy |
| `long_reset_threshold_seconds` outside 5–86400 | invalid threshold |
| `scope=opt_in` with empty platforms | platforms required |
| body-code list longer than 32 | too many codes |
| GET missing / empty / bad JSON | return Default* (`enabled=false`), not 500 |

### 5. Good / Base / Bad Cases

- Good: global off, extra true, soft body `rate_limit_exceeded` → Redis exclude only; next request without sticky binding skips the id.
- Base: empty KV → policy off; OAuth 429 uses current `handle429`.
- Bad: treating `sessionHash != ""` as hard affinity (skips layer-2 on almost every request).

### 6. Tests Required

- Unit: applies / classify (soft vs Anthropic window / Codex 100% / quota death / hard body / long-reset policy); `HandleUpstreamError` does not write reset/temp-unsched on soft; `CheckErrorPolicy` returns none for applicable OAuth 429.
- Handler: GET empty KV → default off; PUT rejects bad ttl; response is not on public settings.
- Cache: Redis key `oauth-soft-429:{id}`; SET TTL; SCAN/merge fail-open.
- Frontend: Settings card save hits only the fleet API; page-level save does not; EditAccountModal extra inherit / force-on / force-off; hidden on API Key.

### 7. Wrong vs Correct

#### Wrong

```go
mergeOAuthFleetSoft429ExcludedIDs(ctx, rl, excluded, sessionHash != "")
```

A generated session hash is not hard affinity. That skips layer-2 on almost every new request (AC2 miss).

#### Correct

```go
mergeOAuthFleetSoft429ExcludedIDs(ctx, rl, excluded, oauthFleetSoft429HasHardAffinity(previousResponseID, stickyAccountID))
```
