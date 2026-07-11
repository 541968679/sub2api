# Data Consistency: Account Pagination, Usage Dimensions, and Model URLs

## Data model

This batch does not change schemas or stored values. It reads existing account
rows and usage-log columns. User model statistics group by trimmed
`requested_model`, falling back to `model` when the requested value is empty.
Token and cost fields remain direct sums of the existing columns.

## Key files

- `backend/internal/repository/account_repo.go`: filtered account list and count.
- `backend/internal/repository/usage_log_repo.go`: requested-model aggregation.
- `backend/internal/service/openai_endpoint_url.go`: version-aware URL helper.
- `backend/internal/service/upstream_models.go`: upstream model discovery URL.
- Focused tests live beside these repository and service files.

## Core flow

Account pagination builds one Ent query, clones it for `Count`, then applies
offset/order/limit to the original builder for `All`. This prevents query
interceptors from mutating the builder used by the subsequent list operation.

`GetUserModelStats` delegates to `getModelStatsWithFiltersBySource` with
`ModelSourceRequested`. The canonical query selects and groups by
`COALESCE(NULLIF(TRIM(requested_model), ''), model)` while keeping the normal
token and cost sums.

OpenAI API-key model synchronization calls
`buildOpenAIEndpointURL(base, "/v1/models")`. Bare hosts receive `/v1/models`,
versioned bases such as `/v2` and `/v4` receive `/models`, and an existing model
endpoint remains unchanged.

## Important mechanisms

- Account list pages must satisfy `pagination.Total == len(items)` when the
  complete result fits on one page.
- User model labels reflect the model the user requested, including after local
  mapping or bridge routing.
- Input/output/cache token values, `total_cost`, stored `actual_cost`, and
  account cost are summed without display or billing transforms.
- Model URL normalization is shared with responses, chat completions, and
  embeddings to avoid endpoint-specific version handling.

## Known pitfalls

- Do not remove `q.Clone()` from the account count path. Ent interceptors can
  append predicates when a query executes and pollute a reused mutable builder.
- Do not replace requested-model grouping with frontend regrouping or derive
  prices from cost/token ratios.
- Do not apply model-sync URL behavior to curated/default model selection,
  routing, scheduling, Claude-GPT bridge logic, or OpenAI Images.
- The `76dd18cb3` alignment baseline contains unrelated RED tests and a missing
  integration `cacheRecorder` fixture. Focused repository and service-file test
  commands are required until those other batches are merged.
