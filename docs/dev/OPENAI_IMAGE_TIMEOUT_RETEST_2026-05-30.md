# OpenAI Image Timeout And Retest Record

> Follow-up record for the `gpt-image-2` non-return / latency investigation.
> This document captures the May 30, 2026 fix, the post-fix retest, and the
> remaining analysis path.

## Scope

- Endpoint: `/v1/images/generations`
- Model: `gpt-image-2`
- Local Sub2API base URL: `http://127.0.0.1:18081`
- Implementation commit: `339a1acf fix: bound openai image generation waits`
- Retest run: `openai-image-retest-after-timeout-20260530-002928`
- Retest artifacts:
  - `output/openai-image-retest-after-timeout-20260530-002928/run.json`
  - `output/openai-image-retest-after-timeout-20260530-002928/requests.jsonl`
  - `output/openai-image-retest-after-timeout-20260530-002928/summary.json`

## Problem Being Solved

The first priority was not to make every image faster. The urgent UX problem was:

1. Fast upstream transport failures could surface as immediate failed image
   requests.
2. Long upstream waits could leave the client with no clear terminal result.

Earlier timing diagnostics showed that a successful slow request spent almost
all server-side time waiting for the upstream image result/body, not in Sub2API
auth, account scheduling, response transformation, or writeback. That made the
first fix boundary clear: make fast failures retryable and make very long waits
bounded and typed.

## Fix Summary

Changed files:

- `backend/internal/service/openai_images_responses.go`
- `backend/internal/service/openai_images_test.go`
- `backend/internal/handler/openai_images.go`
- `docs/dev/codebase/gateway.md`

Runtime behavior added:

- Codex `/responses` OAuth image path retries fast no-header transport failures
  on the same account up to 3 total attempts.
- Retry backoff is short: 250ms before attempt 2, 750ms before attempt 3.
- The full upstream generation wait/body read is bounded by image tier:
  - 1K: 180s
  - 2K: 240s
  - 4K and unknown/non-standard sizes: 360s
- Long no-output waits return typed error `image_generation_timeout` with HTTP
  504 before any non-streaming body is written.
- Retry exhaustion for fast no-header transport failures returns typed error
  `image_generation_upstream_unreachable` with HTTP 502.
- Streaming requests keep the same deadline and emit a typed SSE error if the
  timeout happens after streaming has begun.

Intentional non-goals in this fix:

- No account-level failover was added for long generation waits. The current
  evidence says the long section is upstream generation, and switching accounts
  after several minutes may duplicate cost without clearly improving completion.
- No request-parameter normalization was added. Non-standard sizes are accepted
  and protected by the default 360s timeout bucket.
- No production trace logging is enabled by default. `OPENAI_IMAGE_TRACE_LOG`
  remains opt-in and temporary.

## Local Verification

Focused unit tests run after the code change:

```text
go test -tags=unit ./internal/service -run "OpenAIGatewayServiceForwardImages_OAuth|BuildOpenAIImagesResponsesRequest|CollectOpenAIImagesFromResponsesBody" -count=1
go test -tags=unit ./internal/handler -run "OpenAI" -count=1
```

Coverage added around the fix:

- Retry succeeds after a fast retryable transport error.
- Retry exhaustion returns typed `image_generation_upstream_unreachable`.
- Non-streaming upstream wait deadline returns typed `image_generation_timeout`.
- Handler maps typed image generation errors before the generic fallback path.

## Retest Matrix

The post-fix retest simulated real application pressure instead of a single
request:

| Parameter | Value |
|-----------|-------|
| Total requests | 36 |
| Concurrency | 4 |
| Repeat per combo | 6 |
| Client timeout | 430s |
| Response format | `b64_json` |
| `n` | 1 |
| Prompt | Fixed prompt; only `prompt_sha256_12` recorded |
| Sizes | `1536x1024`, `1440x2560` |
| Qualities | `auto`, `medium`, `high` |

Combinations:

| Combo | Tier | Size | Quality |
|-------|------|------|---------|
| `2k_landscape_auto` | 2K | `1536x1024` | `auto` |
| `2k_landscape_medium` | 2K | `1536x1024` | `medium` |
| `2k_landscape_high` | 2K | `1536x1024` | `high` |
| `4k_portrait_auto` | 4K | `1440x2560` | `auto` |
| `4k_portrait_medium` | 4K | `1440x2560` | `medium` |
| `4k_portrait_high` | 4K | `1440x2560` | `high` |

## Retest Result

Overall:

| Metric | Result |
|--------|--------|
| Success | 36 / 36 |
| HTTP status | 36 x 200 |
| Fast failures under 5s | 0 |
| Client timeouts | 0 |
| Service timeouts | 0 |
| Max observed request time | 65.578s |

Per-combo successful duration distribution:

| Combo | Count | Min | Mean | Stdev | CV | P50 | P90 | P95 | Max |
|-------|------:|----:|-----:|------:|---:|----:|----:|----:|----:|
| `2k_landscape_auto` | 6 | 16.157s | 30.581s | 10.672s | 0.3490 | 36.844s | 38.054s | 38.144s | 38.234s |
| `2k_landscape_medium` | 6 | 14.812s | 34.724s | 10.253s | 0.2953 | 37.219s | 42.298s | 42.540s | 42.782s |
| `2k_landscape_high` | 6 | 25.922s | 36.448s | 5.946s | 0.1631 | 37.148s | 41.539s | 42.699s | 43.859s |
| `4k_portrait_auto` | 6 | 32.000s | 47.529s | 10.789s | 0.2270 | 47.774s | 56.938s | 61.258s | 65.578s |
| `4k_portrait_medium` | 6 | 43.969s | 49.898s | 5.555s | 0.1113 | 48.930s | 55.242s | 57.770s | 60.297s |
| `4k_portrait_high` | 6 | 39.937s | 47.924s | 8.377s | 0.1748 | 45.539s | 56.461s | 60.090s | 63.719s |

One backend trace during the retest captured an
`upstream_transport_retry` event; the corresponding downstream request still
completed successfully:

```text
openai-image-retest-after-timeout-20260530-002928-009-4k_portrait_auto-d0098d
```

This is the important behavioral confirmation: the new same-account retry can
absorb at least some fast upstream transport instability without exposing a
failed image request to the client.

## Interpretation

The 36-request sample is enough to validate the fix path for the observed issue:

- No rapid visible failures were reproduced after adding same-account retry.
- No request entered the new long-wait timeout window in this sample.
- Successful 2K requests clustered roughly under 45s.
- Successful 4K requests clustered roughly under 66s.
- The current production-facing timeout windows remain conservative relative to
  this sample:
  - 2K timeout: 240s
  - 4K timeout: 360s

The sample is not enough to prove a stable normal distribution for each
parameter combination. Each combo has only 6 observations, and upstream image
generation latency can be load-dependent and time-dependent. Treat these
numbers as a safe post-fix smoke/stability sample, not as a final SLO baseline.

## Recommended Timeout Policy For Now

Keep the implemented server-side deadline values:

| Tier | Current timeout | Reason |
|------|-----------------|--------|
| 1K | 180s | Enough margin over normal small-image latency, still avoids indefinite waits. |
| 2K | 240s | More than 5x the observed post-fix P95 for the sampled 2K combos. |
| 4K/unknown | 360s | More than 5x the observed post-fix P95 for the sampled 4K combos and protects non-standard sizes. |

Client-side timeouts should stay higher than the server-side timeout so clients
receive the typed Sub2API error instead of disconnecting first. For the current
server windows, use at least:

- 2K client timeout: 270s or higher
- 4K/unknown client timeout: 390s or higher

## Next Analysis Steps

To turn the timing distribution into an operational baseline, run a larger
sample in one or more production-like windows:

| Goal | Suggested sample |
|------|------------------|
| Estimate normal latency range | 30-50 successful requests per combo |
| Compare quality impact | Same prompt, same size, `auto`/`medium`/`high` balanced |
| Compare size impact | Same prompt, 1K/2K/4K balanced |
| Detect load sensitivity | Repeat during low-traffic and high-traffic windows |
| Separate Sub2API vs upstream | Enable `OPENAI_IMAGE_TRACE_LOG=true` only during the sampling window |

Record for each request:

- `trace_id`
- status code
- error type, if any
- total duration
- upstream header wait
- upstream body/result wait
- downstream write duration
- account ID
- size, quality, and tier

Do not record prompts, image bytes/base64, auth headers, cookies, or API keys in
diagnostic logs.

## Follow-up Questions

The next optimization round should be evidence-driven:

1. If `image_generation_upstream_unreachable` appears frequently, classify the
   underlying transport errors and decide whether retries should include account
   failover or remain same-account only.
2. If `image_generation_timeout` appears frequently but later upstream images
   would have succeeded, consider adding async job/status polling instead of
   simply extending synchronous request timeouts.
3. If long waits correlate with only a subset of accounts, add account-level
   timing metrics before introducing failover.
4. If non-standard sizes dominate timeout cases, decide whether to normalize,
   reject, or explicitly bucket them by megapixels.
