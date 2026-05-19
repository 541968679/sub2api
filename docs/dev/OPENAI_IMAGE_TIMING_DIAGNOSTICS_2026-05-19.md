# OpenAI Image Timing Diagnostics Progress

> Progress record for the `gpt-image-2` image-generation latency investigation.
> This is a temporary diagnostic document, not a permanent product feature spec.

## Current Status

- Date: 2026-05-19
- Scope: `/v1/images/generations` with `model=gpt-image-2`
- Local diagnostic switch: `OPENAI_IMAGE_TRACE_LOG=true`
- Implementation commit: `d1e206f5 feat: add OpenAI image trace logging`
- Local Sub2API base URL used by Huajing: `http://127.0.0.1:18081`
- Local trace logs:
  - `tmp/dev-stack/logs/backend.out.log`
  - `tmp/dev-stack/logs/backend.err.log`

The local diagnostic trace has been implemented and verified. The trace is off
by default and only emits structured timing logs for the target image generation
path and model.

## Trace Events

Each enabled request should emit these events with the same correlation fields:

| Order | Event | Meaning |
|-------|-------|---------|
| 1 | `request_received` | Sub2API accepted the downstream request. |
| 2 | `auth_done` | API key auth and request context setup completed. |
| 3 | `account_slot_acquired` | Account scheduling and concurrency slot acquisition completed. |
| 4 | `upstream_request_start` | Sub2API is about to send the request upstream. |
| 5 | `upstream_headers_received` | Upstream response headers were received. |
| 6 | `upstream_body_read_done` | Sub2API finished reading the upstream response body. |
| 7 | `downstream_response_built` | Sub2API parsed/transformed the result and built the downstream response. |
| 8 | `downstream_write_done` | Sub2API finished writing the HTTP response to Huajing. |
| 9 | `usage_record_submitted` | Usage/billing recording work was submitted. |

Safe fields only are logged: request correlation IDs, account ID, model, size,
quality, stream flag, status code, wall time, elapsed milliseconds, upstream
request ID when available, endpoint, `n`, and multipart flag.

The trace must not log prompts, image bytes/base64, authorization headers,
cookies, API keys, or full request bodies.

## Local Findings

### Base URL Misconfiguration

An early Huajing test hit:

```text
POST /v1/v1/images/generations -> 404
```

This means Huajing likely configured a base URL ending in `/v1` while also
appending `/v1/images/generations`.

Correct local base URL:

```text
http://127.0.0.1:18081
```

### Missing Prompt Case

Two later requests reached Sub2API and selected account `28`, but failed before
the upstream request with:

```text
error="prompt is required"
status_code=502
```

This confirmed that routing, auth, model selection, and account scheduling were
working locally, but Huajing did not send a valid prompt for those attempts.

### Successful Single-Image Baseline

Successful request:

| Field | Value |
|-------|-------|
| `request_id` | `eed367f3-67af-435c-8b08-be75d95a047a` |
| `client_request_id` | `cf6bdbaf-1eb9-400f-9620-cf192058a112` |
| `model` | `gpt-image-2` |
| `size` | `1440x2560` |
| `n` | `1` |
| `account_id` | `28` |
| `status_code` | `200` |
| Sub2API total latency | `184424ms` |

Observed elapsed times:

| Event | Elapsed |
|-------|---------|
| `request_received` | `0ms` |
| `auth_done` | `0ms` |
| `account_slot_acquired` | `22ms` |
| `upstream_request_start` | `32ms` |
| `upstream_headers_received` | `1045ms` |
| `upstream_body_read_done` | `184250ms` |
| `downstream_response_built` | `184386ms` |
| `downstream_write_done` | `184391ms` |
| `usage_record_submitted` | `184398ms` |
| HTTP access log total | `184424ms` |

Derived timing:

| Segment | Calculation | Duration |
|---------|-------------|----------|
| Sub2API pre-upstream work | `upstream_request_start - request_received` | `32ms` |
| Upstream header wait | `upstream_headers_received - upstream_request_start` | `1013ms` |
| Upstream body/result wait | `upstream_body_read_done - upstream_headers_received` | `183205ms` |
| Sub2API parse/build/writeback | `downstream_write_done - upstream_body_read_done` | `141ms` |
| Usage submission after writeback | `usage_record_submitted - downstream_write_done` | `7ms` |

Conclusion from this local baseline: almost all measured server-side time was
spent waiting for the upstream image result/body. Sub2API local preprocessing,
account scheduling, response construction, and writeback were all small compared
with the upstream stage.

## Meaning Of Downstream Writeback

`upstream_body_read_done -> downstream_write_done` means:

1. Sub2API has fully read the upstream image response.
2. Sub2API parses/checks/transforms the response into the OpenAI-compatible
   image response.
3. Sub2API builds the downstream response.
4. Gin/HTTP finishes writing that response to Huajing.

This segment does not prove that Huajing has rendered or saved the image. It
does not include Huajing-side parsing, base64 handling, local save time, UI
rendering, or any client-side completion notification.

## Production Testing Notes

The same trace can be used in production for a short diagnostic window because
it is opt-in and payload-safe.

Recommended production process:

1. Deploy the commit containing the trace.
2. Keep `OPENAI_IMAGE_TRACE_LOG` disabled by default.
3. Enable `OPENAI_IMAGE_TRACE_LOG=true` only during the test window.
4. Send a small number of controlled `gpt-image-2` requests from Huajing.
5. Record Huajing-side click/start time and visible-completion time.
6. Disable the switch after sampling.

If production image traffic is high, add an additional filter before testing,
such as a specific `api_key_id`, user ID, or `client_request_id`, so logs remain
small and easy to correlate.

## Network Latency Boundary

Sub2API server logs alone can precisely measure only the server-side interval:

```text
request_received -> downstream_write_done
```

They cannot, by themselves, measure:

```text
Huajing request start -> production Sub2API request_received
production Sub2API downstream_write_done -> Huajing response done/UI complete
```

To estimate the local-to-production and production-to-local portions, compare
Huajing or proxy timestamps with the production trace:

| Desired segment | Compare |
|-----------------|---------|
| Huajing to production Sub2API | Huajing/proxy `client_request_start` -> production `request_received` |
| Production Sub2API server work | production `request_received` -> production `downstream_write_done` |
| Production Sub2API to Huajing | production `downstream_write_done` -> proxy `client_response_done` or Huajing UI completion |

Simpler rough estimate:

```text
Huajing observed total time - production Sub2API server-side total time
= client/network/UI extra time
```

This rough difference includes request upload, response download, Huajing
parsing/saving/rendering, and any client-side queueing.

## Next Steps

- Run a production single-image request and compare Huajing wall-clock time with
  production `request_received -> downstream_write_done`.
- Run a Huajing four-image test to determine whether Huajing sends four
  parallel requests, four serial requests, or one `n=4` request.
- If the rough client/network/UI extra time is large, capture the real request
  with Fiddler or mitmproxy and compare proxy timestamps against production
  trace events.
- Keep `OPENAI_IMAGE_TRACE_LOG` disabled outside targeted diagnostic windows.
