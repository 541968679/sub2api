# OpenAI wait-timeout: client vs Ops message

## Scenario: header-wait / first-useful-frame timeout must not leak internal markers

### 1. Scope / Trigger
- Trigger: OpenAI header-wait (`openai_header_wait_timeout`) or first-useful-frame (`openai_first_useful_frame_timeout`) fires.
- Includes Claude-GPT bridge / Anthropic `/v1/messages` failover replay and native OpenAI Chat Completions / Responses SSE after commit.
- Does not change timeout seconds, failover timing, or inbound protocol.

### 2. Signatures
- `OpenAIWaitTimeoutClientMessage() string`
- `rewriteOpenAIWaitTimeoutClientText(s string) string`
- `IsOpenAIWaitTimeoutOpsError(message, upstreamMessage, body string) bool`
- `OpenAIGatewayService.newOpenAIStreamFailoverError(...) *UpstreamFailoverError`
- `OpenAIGatewayHandler.mapAnthropicFailoverBodyError(...) (int, string, string, bool)`

### 3. Contracts
- Client JSON / SSE `error.message` and `error.code` = `Upstream service temporarily unavailable` (same as `mapUpstreamError(502)`).
- Client text must not contain `openai_header_wait_timeout`, `openai_first_useful_frame_timeout`, or `waited_ms=`.
- Ops `event.Message`, `RawUpstreamBody`, and committed-abort `error.Error()` keep `openai_*_timeout waited_ms=N`.
- `recordOpsUpstreamAttempt` drops generic 502 sentences. Never put the client sentence in `event.Message` for these hops.
- Anthropic exhausted replay reads `ResponseBody`; that wrapper is client-only. Ops re-record uses `FailoverOpsRawBody` → `RawUpstreamBody`.

### 4. Validation & Error Matrix
| Path | Downstream | Ops |
| --- | --- | --- |
| Uncommitted header wait | Failover body client 502; no write yet | Marker in `event.Message` + `RawUpstreamBody` |
| Uncommitted first useful frame | Same | Same |
| Anthropic / bridge exhausted replay | JSON or `event: error` with client 502 | Marker from `RawUpstreamBody` |
| Committed first useful frame | SSE error with client 502 | `error.Error()` still has marker; no account switch |

### 5. Good/Base/Bad Cases
- Good: bridge client sees `Upstream service temporarily unavailable`; Ops row still matches `IsOpenAIWaitTimeoutOpsError`.
- Base: native OpenAI exhausted 502 already used the same sentence; wait-timeout now matches it.
- Bad: putting the marker in `ResponseBody.error.message` so `handleAnthropicFailoverExhausted` replays it. Bad: setting `event.Message` to the generic 502 so `recordOpsUpstreamAttempt` clears it.

### 6. Tests Required
- Failover `ResponseBody` hides markers; `RawUpstreamBody` / Ops context keeps them.
- `handleAnthropicFailoverExhausted` JSON and `event: error` hide leftover marker bodies.
- Committed SSE body hides markers; `error.Error()` still classifies as wait-timeout.
- `ClassifyOpsErrorRateCalibers` Recovered wait-timeout still counts in schedule, not user error rate.

### 7. Wrong vs Correct
#### Wrong
Reuse one string for client and Ops. Bridge replay then leaks `openai_header_wait_timeout waited_ms=90001`, or Ops loses the marker because the generic 502 is cleared.
#### Correct
`OpenAIWaitTimeoutClientMessage()` for every client field. Marker stays in Ops-only fields. `mapAnthropicFailoverBodyError` rewrites leftover marker bodies as a second gate.
