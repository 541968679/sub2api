# InvokeAI Canvas PoC

## Data model

This PoC does not add Sub2API database tables, migrations, services, or frontend
routes. InvokeAI keeps its own SQLite database, generated images, model cache,
and runtime configuration under an external root directory.

The external InvokeAI checkout now stores OpenAI-compatible external provider
credentials per InvokeAI user in its own SQLite table,
`user_external_provider_configs`. API keys are stored in plaintext for the PoC;
only configured status and base URL are returned to the frontend.

Local paths:

- InvokeAI source checkout: `E:\cursor project\InvokeAI`
- InvokeAI runtime root: `E:\cursor project\invokeai-sub2api-poc`
- InvokeAI config: `E:\cursor project\invokeai-sub2api-poc\invokeai.yaml`

## Key files

- `E:\cursor project\InvokeAI\README.md` - upstream overview.
- `E:\cursor project\InvokeAI\docs\src\content\docs\start-here\manual.mdx` -
  upstream manual install instructions.
- `E:\cursor project\InvokeAI\docs\src\content\docs\configuration\invokeai-yaml.mdx` -
  upstream runtime config documentation.
- `E:\cursor project\InvokeAI\docs\src\content\docs\features\External Models\openai.mdx` -
  upstream OpenAI-compatible external image model setup.
- `E:\cursor project\InvokeAI\invokeai\app\services\session_processor\session_processor_default.py` -
  InvokeAI queue worker pool that executes queued sessions.
- `E:\cursor project\InvokeAI\invokeai\app\services\session_queue\session_queue_sqlite.py` -
  SQLite-backed queue state and status transitions.
- `E:\cursor project\InvokeAI\invokeai\app\api\routers\session_queue.py` -
  queue API, including the current-item compatibility endpoints.
- `E:\cursor project\InvokeAI\invokeai\frontend\web\src\services\api\endpoints\queue.ts` -
  React RTK query bindings for queue/current items.
- `E:\cursor project\InvokeAI\tests\app\services\test_session_processor_parallel.py` -
  focused regression tests for queue concurrency.

## Core flow

1. Sub2API continues to run as the OpenAI-compatible API gateway on
   `http://127.0.0.1:18081`.
2. InvokeAI runs as an independent canvas/image UI on `http://127.0.0.1:9090`.
3. InvokeAI external OpenAI settings point at Sub2API:
   `external_openai_base_url: http://127.0.0.1:18081`.
4. InvokeAI sends image generation/edit requests to Sub2API with a Sub2API user
   API key saved on the current InvokeAI user.
5. External OpenAI image generation reads `queue_item.user_id`, fetches that
   user's provider config, and sends the request using that key/base URL.
6. For GPT image models, InvokeAI first uses Sub2API's OpenAI-compatible
   `/v1/images/generations` or `/v1/images/edits` route. If a non-OpenAI base
   URL returns Sub2API's `502` `upstream_error` from that Images route, InvokeAI
   retries once through `/v1/responses` with an `image_generation` tool payload;
   this uses Sub2API's existing Responses image bridge.
7. Sub2API routes/bills the request through its existing gateway/account stack.
8. Multiple queued InvokeAI sessions may now run at the same time. In the API-only
   deployment this means multiple external OpenAI/Sub2API image requests can be
   in flight concurrently instead of waiting behind a single local queue worker.

## Important mechanisms

- InvokeAI is intentionally outside this repository and should not be imported
  or vendored into Sub2API.
- Runtime data is also outside the Sub2API repository so generated images,
  SQLite files, model caches, and node packs do not pollute this checkout.
- The local config enables InvokeAI native multiuser mode with
  `multiuser: true` and `strict_password_checking: true`.
- The local PoC uses a built-in administrator instead of exposing a first-run
  setup flow. Local credentials are `admin` / `admin123`. For cloud deployment,
  change `builtin_admin_password` to a strong password or disable the local
  built-in-admin config and provision an admin out-of-band.
- Normal local start/restart/stop for InvokeAI should use the local script in
  the InvokeAI checkout. It fixes `host: 127.0.0.1`, `port: 9090`, multiuser
  settings, built-in admin settings, UTF-8 config encoding, process tracking,
  and log paths:

```powershell
cd "E:\cursor project\InvokeAI"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 restart
```

or:

```bat
scripts\dev-stack.cmd restart
```

- In multiuser mode, the External Providers UI writes the current user's
  provider config instead of `api_keys.yaml`. Single-user mode keeps the old
  YAML-backed path.
- The InvokeAI checkout used by this PoC now has a configurable session queue
  worker pool. `session_queue_concurrency` defaults to `4`; startup wires this
  value into `DefaultSessionProcessor`. This is intentionally global queue
  concurrency, not a local-GPU-only path.
- The SQLite queue `dequeue()` operation atomically promotes one pending row to
  `in_progress`, then returns the full queue item. This prevents multiple worker
  threads from claiming the same item.
- Queue status remains backward compatible through the old single-current-item
  fields, but new code should prefer `GET /api/v1/queue/{queue_id}/current_items`
  when it needs all active items.
- Frontend progress/cancel code now reads all active current items and checks
  destination/user ownership instead of assuming only one active item exists.
- Queue clear/cancel paths were adjusted for multi-current behavior. Non-admin
  queue clear/cancel actions stay scoped to that user's items and must not
  interrupt another user's in-progress request.
- External generation request transforms must preserve `user_id`. The OpenAI
  provider configuration lookup is per InvokeAI user in multiuser mode; dropping
  `ExternalGenerationRequest.user_id` during capability refresh or size
  bucketing makes generation fail locally with
  `OpenAI provider is not configured for this user` even when the correct
  user's OpenAI/Sub2API provider config exists.

## Verification

Last verified: 2026-05-31.

Backend focused tests:

```powershell
cd "E:\cursor project\InvokeAI"
.\.venv\Scripts\python.exe -m pytest tests/app/services/test_session_processor_parallel.py tests/app/services/session_queue/test_session_queue_status_sequence.py tests/app/services/session_queue/test_session_queue_status_event_isolation.py tests/app/services/session_queue/test_session_queue_clear.py tests/app/services/external_generation/test_external_provider_adapters.py -q
```

Result: `31 passed, 2 warnings in 5.56s`.

External provider context regression tests:

```powershell
cd "E:\cursor project\InvokeAI"
.\.venv\Scripts\python.exe -m pytest tests/app/services/external_generation/test_external_generation_service.py tests/app/services/external_generation/test_external_provider_adapters.py tests/app/invocations/test_external_image_generation.py -q
```

Result: `30 passed, 6 warnings`.

Frontend type check:

```powershell
cd "E:\cursor project\InvokeAI\invokeai\frontend\web"
pnpm run lint:tsc
```

Result: exit code `0`.

## Known pitfalls

- The InvokeAI source/dev install needs the React UI built into
  `invokeai/frontend/web/dist`; otherwise the backend starts without a UI.
- InvokeAI external OpenAI models are not arbitrary free-form UI entries in this
  fork. They are provided by `STARTER_MODELS` plus the OpenAI provider model
  set. `gpt-image-2` is supported by the fork as
  `external://openai/gpt-image-2`.
- OpenAI GPT Image starter models intentionally do not declare
  `aspect_ratio_sizes` or `resolution_presets`. That keeps InvokeAI's width and
  height controls editable and prevents the external generation service from
  bucketing custom Sub2API sizes back to preset resolutions. A `4096x4096`
  `max_image_size` guard remains in place.
- For Sub2API-backed OpenAI configuration in InvokeAI, set Base URL to the
  gateway origin, for example `https://zerocode.kaynlab.com`, without `/v1`.
  The InvokeAI provider appends `/v1/images/generations` and `/v1/images/edits`
  itself, so `https://zerocode.kaynlab.com/v1` would become `/v1/v1/...`. The
  same origin is also used for the guarded `/v1/responses` fallback when remote
  Sub2API reports an Images-route upstream failure.
- Do not use Sub2API's forbidden local ports. InvokeAI uses `9090` for this PoC,
  leaving Sub2API backend/frontend on `18081` and `15174`.
- External starter model records remain instance-level. Deleting one user's
  provider config does not remove external model records, because other users
  may still rely on them.
- The old `GET /current` and scalar queue status fields still expose one current
  item for compatibility. UI or automation that needs accurate multi-image
  progress must use `current_items` plus destination/user filtering.
- `session_queue_concurrency=4` is a starting point for API-only deployments.
  Increase only after measuring Sub2API account capacity, upstream rate limits,
  and browser/client timeout behavior.
