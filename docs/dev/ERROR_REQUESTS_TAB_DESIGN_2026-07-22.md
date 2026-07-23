# 管理员错误请求 Tab — Design

日期：2026-07-22  
PRD：`ERROR_REQUESTS_TAB_PRD_2026-07-22.md`

## 1. Architecture

```
UsageView (admin)
  activeTab === 'errors'
    → hide usage charts / usage table / ranking / export-cleanup
    → ErrorRequestFilters (local filter state + shared date range)
    → ErrorRequestStatsCards  ← GET /admin/ops/errors/stats
    → OpsErrorLogTable        ← GET /admin/ops/errors (extended query)
    → OpsErrorDetailModal
```

## 2. API

### 2.1 Extend `GET /admin/ops/errors`

Existing filters plus:

| Query | Notes |
|-------|--------|
| `user_id` | exact |
| `user_query` | email ILIKE (already in filter struct) |
| `model` | requested_model OR model (exists) |
| `upstream_model` | exact match on upstream_model |
| `bridge` | `all` \| `bridge` \| `non_bridge` |
| `error_type` | error_type exact |
| `status_codes` | comma list (exists) |
| `q` | message/request_id (exists) |

Response: add `is_claude_gpt_bridge: boolean` on each row (computed).

### 2.2 New `GET /admin/ops/errors/stats`

Same query params as list (no page). Response:

```json
{
  "success_requests": 1200,
  "terminal_error_requests": 80,
  "terminal_error_requests_filtered": 50,
  "raw_error_rows": 320,
  "total_requests": 1280,
  "error_rate": 0.0390625,
  "top_status_codes": [{"key": "502", "count": 40}, ...],
  "top_requested_models": [{"key": "claude-haiku-...", "count": 30}, ...],
  "top_upstream_models": [{"key": "gpt-5.4", "count": 28}, ...]
}
```

### 2.3 Rate formula (S1)

- \(F_{biz}\): time, user, group, account, platform, model, upstream_model, bridge, view  
- \(F_{err}\): status_codes, error_type, q  

```
terminal_filtered = DISTINCT request_key in errors under F_biz ∧ F_err
terminal_all_biz  = DISTINCT request_key in errors under F_biz
success           = usage rows under F_biz (token/dialog only)
error_rate        = terminal_filtered / (success + terminal_all_biz)
```

`request_key = COALESCE(NULLIF(trim(request_id),''), NULLIF(trim(client_request_id),''), id::text)`

### 2.4 Bridge heuristic (SQL + Go)

```
platform IN ('antigravity','anthropic')
AND lower(upstream_model) LIKE 'gpt-%'
```

### 2.5 Success usage scope

`usage_logs` where:

- same time/user/group/account
- model/requested_model/upstream filters when set
- platform via `JOIN accounts` when set
- bridge heuristic on requested/upstream when set
- `COALESCE(image_count,0)=0 AND COALESCE(video_count,0)=0`

## 3. Frontend

- Tab switch: `v-if`/`v-show` hide usage-only sections when `errors`
- Dedicated error filters component; do not reuse UsageFilters for errors
- Stats cards with rate coloring: ≥20% danger, ≥5% warn
- Table: ensure user + bridge badge + mapping columns visible

## 4. Route order

Register `GET /errors/stats` **before** `GET /errors/:id`.

## 5. Tests

- `buildOpsErrorLogsWhere` bridge / upstream_model
- Stats formula unit test with mocked counts or SQL builder test
- UsageView tab isolation + stats load
