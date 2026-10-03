# OpenAI API-Key Upstream Routing

## Scenario: passthrough + dual-probe

### 1. Scope / Trigger
- Trigger: changing how OpenAI API-key accounts pick `/v1/responses` vs `/v1/chat/completions`.

### 2. Signatures
- `openai_compat.ResolveUpstreamAPI(inbound, extra) UpstreamEndpoint`
- `openai_compat.ShouldUseResponsesAPI(extra)` = inbound CC only
- extra: `openai_responses_mode`, `openai_responses_supported`, `openai_chat_completions_supported`

### 3. Contracts
- `passthrough`: inbound = upstream. Probe does not retarget.
- `auto` + Responses supported/unknown: inbound CC still converts to Responses.
- `openai_chat_completions_supported` never changes auto.
- Illegal/missing mode → `passthrough`. Following probes requires an explicit `"auto"` string in extra. Never treat a missing key as auto.
- Probe model: see Scenario: capability-probe model + targeted reprobe.
- Create/edit UI defaults to `passthrough` and always persists the selected mode, including `"auto"`.

### 4. Validation & Error Matrix
- Probe transport error → do not write that key.
- Update without credentials → do not re-probe.
- passthrough + upstream 404 → ordinary upstream error, do not change mode.

### 5. Good/Base/Bad Cases
- Good: explicit `passthrough` → CC→CC and Responses→Responses.
- Base: missing extra → passthrough (CC→CC, Responses→Responses).
- Bad: missing extra still converting inbound CC to Responses.
- Bad: auto + both probes true silently becomes passthrough.

### 6. Tests Required
- Table: mode × inbound × Rsupp × CCsupp → upstream.
- Dual probe URLs + one `UpdateExtra`.
- Gateway inbound CC and inbound Responses.

### 7. Wrong vs Correct
#### Wrong
Use `ShouldUseResponsesAPI` for inbound Responses. passthrough + Rsupp false goes to the CC bridge.
#### Correct
`Forward` uses `ResolveUpstreamAPI(InboundResponses, extra)`.

## Scenario: kimi native chat completions

### 1. Scope / Trigger
- Trigger: forwarding `kimi-*` / platform `kimi` to an upstream that only accepts native `/v1/chat/completions`.

### 2. Signatures
- `kimiRequiresNativeChatCompletions(account, models...) bool`
- Checked in `ForwardAsChatCompletions` and `Forward` before `ResolveUpstreamAPI`.

### 3. Contracts
- Platform `kimi`, or any requested/billing/upstream model `kimi` or `kimi-*`, always uses raw Chat Completions.
- Inbound `/v1/chat/completions` is not converted to Responses, including `force_responses` and auto+unknown/yes.
- Inbound `/v1/responses` is converted to a messages body and posted to `/v1/chat/completions`.
- Other models keep the existing `(inbound, extra)` route. `force_responses` still wins for `gpt-*`.

### 4. Validation & Error Matrix
- Missing model on the raw path is unchanged (`model is required`).
- Responses→CC conversion errors stay 400 `invalid_request_error`.

### 5. Good/Base/Bad Cases
- Good: `kimi-k3` + `force_responses` inbound CC → `/v1/chat/completions` + `messages`.
- Good: `kimi-k3` inbound Responses → `/v1/chat/completions` + `messages`.
- Base: `gpt-5.4` + missing extra → Chat Completions.
- Base: `gpt-5.4` + explicit `auto` + missing probe → Responses.
- Bad: treating every Moonshot base URL as CC when the model is not kimi.

### 6. Tests Required
- `TestKimiRequiresNativeChatCompletions`
- `TestForwardAsChatCompletions_KimiK3StaysOnChatCompletions`
- `TestForward_KimiK3InboundResponsesUsesChatCompletions`

### 7. Wrong vs Correct
#### Wrong
Leave auto+unknown / `force_responses` converting kimi-k3 Chat Completions into `/v1/responses`.
#### Correct
Model or platform match short-circuits to the raw chat-completions upstream.

## Scenario: kimi-k3 adaptive validation

### 1. Scope / Trigger
- Trigger: an admin turns on local checks for Kimi K3 Chat Completions when an upstream accepts parameters the official Kimi K3 API rejects.

### 2. Signatures
- Setting key `kimi_k3_adaptive_validation_enabled`. Only the stored string `true` enables it.
- `AdaptKimiK3ChatBody(body []byte) (out []byte, rejectMessage string, changed bool)`
- `OpenAIGatewayService.forwardAsRawChatCompletions` calls it after model mapping and the GLM reasoning-effort normalizer, and before fast policy and `GetAccessToken`.

### 3. Contracts
- Default off. A nil setting service, a missing key, a read error, and any value other than `true` leave the body unchanged.
- Check the upstream model first. Read the switch only when that model, after case folding and one `vendor/` strip, is exactly `kimi-k3`. `a/b/kimi-k3` does not match.
- `kimi-k2`, other vendors, Grok, and `/v1/responses` never enter this checker.
- A non-empty `rejectMessage` is HTTP 400 `invalid_request_error`. The upstream transport is not called. The returned error keeps the `non-streaming openai protocol error:` prefix so the OpenAI handler does not append a second body.
- Absent fields are skipped. Do not read `max_tokens`. Do not rewrite `reasoning_effort=medium` into `high`.
- `max_completion_tokens > 1048576` rejects. `== 1048576` rejects only when `messages` is a non-empty array. Smaller budgets are not estimated.
- `top_logprobs` integers 0 through 20 with JSON `logprobs: true` pass, including 1. Do not invent logprob arrays.
- Function names match `^[a-zA-Z_][a-zA-Z0-9-_]{0,127}$`. 128 characters is legal. Do not walk message history.
- A string content-part `image_url` becomes `{"url": <that string>}`. Object `image_url` and both `video_url` forms stay.
- Admin system settings only. Not on `GET /api/v1/settings/public`. Admin save refreshes the 60 second cache immediately.

### 4. Validation & Error Matrix
- `max_completion_tokens exceeds kimi-k3 maximum 1048576`
- `max_completion_tokens 1048576 plus a non-empty prompt exceeds the 1048576 context window`
- `prediction.type must be content`
- `reasoning_effort must be low, high, or max`
- `stop string exceeds 32 bytes`
- `stop supports at most 5 strings`
- `stop must be a string or an array of strings`
- `top_logprobs must be an integer from 0 to 20`
- `top_logprobs requires logprobs=true`
- `function name must match ^[a-zA-Z_][a-zA-Z0-9-_]{0,127}$`
- The first failed rule wins. A reject does not also rewrite `image_url`.

### 5. Good/Base/Bad Cases
- Good: switch on, upstream model `kimi-k3`, `reasoning_effort=medium` → 400, no upstream request.
- Good: switch on, string `image_url` → upstream sees `{"url":...}`; string `video_url` stays a string.
- Base: switch missing → `kimi-k3` body forwarded unchanged, including `reasoning_effort=medium`.
- Base: switch on, mapped upstream model `gpt-5.4` → illegal body forwarded unchanged.
- Bad: applying the checker to every raw Chat Completions request.
- Bad: rejecting `top_logprobs=1` or a 128-character function name.
- Bad: turning the switch on in code or in a migration. Production stays off until an admin saves `true`.

### 6. Tests Required
- `TestSettingService_KimiK3AdaptiveValidationDefaultOff`
- `TestKimiK3AdaptiveChatBody`
- `TestKimiK3AdaptiveGateway`
- Settings view submits `kimi_k3_adaptive_validation_enabled`.

### 7. Wrong vs Correct
#### Wrong
Reject inside a general OpenAI validator, or synthesize `logprobs` content the upstream did not return.
#### Correct
One pure function, one default-off admin switch, and a `kimi-k3` model predicate on the raw Chat Completions path.

## Scenario: capability-probe model + targeted reprobe

### 1. Scope / Trigger
- Trigger: OpenAI API Key dual-probe must not treat `gpt-4o-audio-preview` / realtime mapping values as the probe model. False `openai_responses_supported=false` sends inbound CC down raw CC + sync-inbound SSE.

### 2. Signatures
- `selectResponsesProbeModel(*Account) string` — used by both probe POSTs
- `firstSortedMappingProbeModel(*Account) string` — old sort-first selector; eligibility only
- `NeedsOpenAICapabilityReprobe(*Account) bool`
- `AccountTestService.ReprobeOpenAIAPIKeysNeedingCapabilityReprobe(ctx, dryRun, allAPIKeys bool) (*OpenAICapabilityReprobeResult, error)`
- `POST /api/v1/admin/accounts/openai-capability-reprobe`

### 3. Contracts
- Probe model: skip mapping values containing `audio` or `realtime` (case-insensitive). Prefer exact `DefaultTestModel` (`gpt-5.4`). Else `sort(chat)[0]`. Else `gpt-5.4`. Do not ban all `gpt-4o`.
- Eligibility: openai + apikey + old sort-first model is audio/realtime + (`openai_responses_supported==false` OR `openai_chat_completions_supported==false`). Missing / non-bool keys = unknown, do not reprobe. Already-true flags do not reprobe.
- Request: `{ "dry_run": bool, "all_apikeys": bool }`. Omitted / empty body → `dry_run=true`, `all_apikeys=false`.
- Response `data`: `{ dry_run, all_apikeys, count, accounts: [{ account_id, name, old_probe_model, new_probe_model, openai_responses_mode, openai_responses_supported, openai_chat_completions_supported, needs_openai_capability_reprobe }] }`. Flags are `*bool` (null if missing). No credentials. Execute reloads extra after each probe so flags are post-probe.
- `all_apikeys=false` lists/probes Q1=B rows only. `all_apikeys=true` lists/probes every OpenAI API Key from `ListAllWithFilters`.
- Execute (`dry_run=false`) calls existing `ProbeOpenAIAPIKeyResponsesSupport` serially. Writes only the two support keys. Does not write `openai_responses_mode`, credentials, schedulable, or status.
- List via `ListAllWithFilters(openai, apikey, status="")`, not `ListByPlatform` (active-only).

### 4. Validation & Error Matrix
- Invalid JSON body → 400.
- Transport failure on one endpoint → do not write that key (unchanged).
- `accountTestService` nil → error, no probe.
- passthrough / `force_*` accounts may still appear if eligible; flags update, mode does not.

### 5. Good/Base/Bad Cases
- Good: mapping has `gpt-4o-audio-preview` + `gpt-5.4` → probe `gpt-5.4`.
- Good: dry-run lists zhima-shaped accounts; execute probes only those IDs.
- Base: no mapping / only wildcards → probe `gpt-5.4`; not eligible (old selector is also `gpt-5.4`).
- Bad: sort-first among all values including audio → false negative on Responses.
- Bad: reprobe because a key is missing (unknown).

### 6. Tests Required
- `TestSelectResponsesProbeModel`: audio+`gpt-5.4` → `gpt-5.4`; audio-only → `gpt-5.4`; empty/wildcard → `gpt-5.4`.
- `TestNeedsOpenAICapabilityReprobe`: zhima-shaped true; tokenbits / missing keys false; audio present but not sort-first false; ccsupp-only false true.
- `TestAccountTestService_ListAndReprobe`: dry-run no HTTP; execute uses `gpt-5.4` body; `UpdateExtra` only two keys.
- `TestAccountHandler_OpenAICapabilityReprobe`: empty body dry-run; execute only eligible IDs; no credentials in JSON.

### 7. Wrong vs Correct
#### Wrong
`sort.Strings(all mapping values)[0]` as the probe model (audio/realtime sorts first).
#### Correct
Filter `audio`/`realtime`, prefer `gpt-5.4`, keep `decide*` and `ResolveUpstreamAPI` unchanged. Eligibility uses the old sort-first model so the new selector cannot hide the false-negative set.
