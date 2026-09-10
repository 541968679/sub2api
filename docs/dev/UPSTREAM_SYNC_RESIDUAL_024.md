# Residual list after Phase 1 leftovers + Phase 2 overlay on `sync/main-1`

Date: 2026-09-10  
Rehearsal HEAD parent: `e8fd01ffb`  
Freeze: `upstream/main` `98d86915b` / 0.2.4  
Not merged to real `main`, not pushed, not deployed. VERSION stays `0.1.287`.

## Landed this overlay (named TestXxx)

| ID | Named test |
|----|------------|
| N24-m404 leftover | `TestClassifySelectionFailureError_ModelNotFoundIsNotOverriddenByRateLimited` (+ CallSiteChain / StillUpgrades) |
| N24-empty-tc | `TestForwardAsRawChatCompletions_StripsEmptyToolCallIdentity` |
| N-proto orphan replay | `TestBuildOpenAIWSCurrentTurnRetryPayloadRejectsOrphanToolOutput` |
| N-pool | `TestGatewayCompatPoolMode429AllowsSameAccountRetry` |
| N-compact deadline | `TestSameAccountRetryAllowedUsesDeadlineInsteadOfPoolCount` |
| N-compact helper | `TestPrepareOpenAICompactFallbackRetryLegacyPathAndSingleAttemptGuard` |
| N-compact Forward hook | `TestOpenAIGatewayForwardRetriesExplicitNativeCompactHTTPFailureOnce` |
| N-id default-off | `TestResolveCodexFingerprintIDsFromRequest_DefaultIsOff` (+ ExplicitOptInHonored) |
| N-guard | `TestOpenAIGatewayService_GuardianParentAffinitySelectsParentAccountAcrossSchedulers` |
| N-team | `TestTeamLinkedError_FanoutMarksSameTeamAccounts` |
| N-img streaming | `TestImagesOAuthStreaming_TextFallbackReturnsCapabilityError` / `TestImagesOAuthStreaming_SplitSafetyRefusalReturns400` |
| N24-b64 | `TestImagesURLToB64JSONEnabled` / `TestBackfillOpenAIImagesB64JSON_RejectsPrivateHosts` / `TestOpenAIGatewayServiceForwardImages_APIKeyBackfillsB64JSONFromURL` |
| N24-imgcd | `TestOpenAIImagesRejectedDriverDoesNotCoolImageModel` |
| N-compact SSE | `TestOpenAIGatewayForwardNonStreamCompactRetryRecordsAttemptWithManagedProxy` / `TestOpenAIGatewayForwardRetriesExplicitNativeCompactSSEFailureBeforeOutput` |
| N-compact stream SSE | `TestOpenAIGatewayForwardRetriesStreamingCompactFailureBeforeOutput` |
| N24-m404 exhaustion | `TestOpenAIManagedSingleAccountModelNotFoundExhaustionPreservesStructured400` |
| N-compact exhaust | `TestOpenAIGatewayForwardDoesNotRecurseWhenCompactFallbackAlsoFails` |
| N-compact passthrough | `TestOpenAIPassthroughCompactFallbackSecondStreamFailureUsesStandardErrorPath` |
| N-compact native-v2 mark | `TestOpenAIGatewayForwardRetriesStreamingCompactAfterNativeV2ContextWithoutPreMark` |
| N24-ws429 replacement | `TestOpenAIWSHTTPBridgeLaterTurn429RetriesCurrentTurnOnReplacementAccount` |

## Residual (not claimed landed)

### Phase 1 leftovers

- `TestPassthroughLifecycle_LaterTurnPreOutputRateLimitRequestsReconnect` — remaining WS session stack; later-turn 429 **before write** and replacement-account current-turn retry are landed.
- N24-created / N24-think / N24-cancel — still no stable `TestXxx` on fork.

### Phase 2 remaining wiring

- Fingerprint **default off** resolver exists; outbound header/body rewrite + SQL seed backfill (upstream dual `225_backfill_codex_fingerprint_seed.sql`) **not** applied. Do not copy filename 225.
- Guardian affinity is applied on HTTP Responses + WS first-message context; profit-control stays default-off.

### Brandon 2026-09-10

- GPT Image 2.5 LiteLLM 单价：**不采用**（生图账单继续 gpt-image-2）。
- OpenAI Fast 写入账单 / 分组强制 Fast / 免费 Fast 按标准档扣费：**不采用**。
- 渠道时段/工作日/倍率价、分组长上下文阶梯价、dash 分组定价快照进计价器：**不采用**。
- 计价相关后续同步默认跳过，除非 Brandon 另说。

### Phase 3 B remaining

- N24-img25 GPT Image 2.5 catalog/pricing (`TestGPTImage25PricingDoesNotUseLegacyImageRates`) — **Brandon 否决改价**；fork catalog stays gpt-image-2。
- N-dash `TestAPIKeyAuthSnapshotGroupPricingRoundtrip` — **Brandon 否决改价**（`LongContextPricingEnabled` + group `ModelPricing` 会改 stored billing）。
- N-ops list-return vitest unnamed; N24-reqid SQL remap **≠ 232**; N24-acctlist DTO; N24-astra capability increment.

### Phase 4–6 C (stop-and-ask / residual, not silent adopt)

- **C-fast** stored `service_tier` / group force+free Fast — **Brandon 否决改价**
- **C-price** channel time/weekday/multiplier prices — **Brandon 否决改价**
- **C-rollup** group usage daily rollup — remap; do not steal 222/223
- **C-grok** wholesale 4.6 / xhigh / media
- **C-cmv2** channel-monitor v2
- **C-plaza** model plaza
- **C-cn** / **C-minimax** / Kimi / Zhipu / Ollama Cloud
- **C-plugin** OAuth outbound plugins
- **C-allowlist** breaking group model allowlists
- **C-fable51** claude-fable-5.1
- **C-simple** simple-mode grouping
- **C-deploy** Go 1.27 / GHCR path swap — 延后
- **C-passkey** never
- **C-agent** Agent Identity last — must overlay on `chatgpt_session_token`; forbid wholesale `CreateAccountModal.vue` / deleting session refresh

### Never this campaign

- Merge `sync/main-1` into real `main`
- `git push` / production deploy
- Set VERSION to 0.2.4
- `git merge upstream/main`
- Commit historical SQL 196–220 CRLF noise
