# Residual list after Phase 1 leftovers + Phase 2 overlay on `sync/main-1`

Date: 2026-09-10  
Rehearsal HEAD parent: `e8fd01ffb`  
Freeze: `upstream/main` `98d86915b` / 0.2.4  
Not merged to real `main`, not pushed, not deployed. VERSION stays `0.1.287`.

## Landed this overlay (named TestXxx)

| ID | Named test |
|----|------------|
| N24-created | `TestChatCompletionsResponseToResponses_CarriesCreatedAt` / `TestAnthropicToResponsesResponse_StampsCreatedAt` / `TestAnthropicEventToResponsesStream_CreatedAtStableAcrossEvents` / `TestWriteOpenAICompactSSEFailureMessage_CarriesCreatedAt` |
| N24-think | `TestAnthropicToChatCompletionsRequest_ThinkingBecomesReasoningContentOnToolTurn` / `TestAnthropicEventToResponses_ThinkingAfterTextKeepsMessageOutput` |
| N24-cancel | `TestForwardAsChatCompletions_CancelsUpstreamBeforeClosingBody` |
| N-id rewrite | `TestApplyCodexFingerprintHeaders_DeviceMode` (+ Off/Session/Full); seed SQL 224 |
| N24-ops list | vitest `returns to the source error list and keeps filters` |
| N24-reqid | `TestUpstreamRequestIDFromHeaders_ReadsOnlyConfiguredHeader` / `TestBuildUsageLogBestEffortInsertQuery_IncludesUpstreamRequestIDWithoutChangingActualCost` |
| N24-acctlist | `TestAccountListLiteKeepsQualityAndSmartScheduleColumns` |
| N24-astra | `TestDefaultModelsIncludeGPT6Astra` |
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
| N24-grok46 | `TestDefaultModelsIncludesGrok46` / `TestClampGrokReasoningEffortValue_PreservesXHighForGrok46` / `TestPatchGrokResponsesBodyPreservesXHighForGrok46` / `TestGrokChatResponsesRuntimeEligibility` / `TestGrokRetryableOnSameAccount_CapacityAndRateLimit` |
| N24-plaza | `TestListPlazaGroups_UsesDisplayPricesNotCostPerToken` / `TestFilterPlazaVisibleGroups_AnonymousSeesOnlyNonExclusive` / vitest `renders display prices for public groups` |
| N24-cmv2 | `TestChannelMonitorQuotaModeRoundTrip` / `TestChannelMonitorV2QueryListSupportsRepeatedAndCommaValues` / `TestChannelMonitorV2ScopeFilterUsesAvailableGroupsForOrdinaryUser` |
| N24-plugin | `TestPluginManagerRoutingDoesNotTouchAPIKeyOrOtherProviders` / `TestPluginPackageInstallerRejectsUnsignedPackageByDefault` / `TestOpenAIGatewayPluginRoutingPreservesAPIKeyAndFailsClosedForOAuth` |
| N24-fable51 | `TestDefaultModelsContainsClaudeFable51` / `TestCLICurrentVersionSatisfiesFable51Gate` / `TestGetModelDefaultPricing_ReturnsFable51CacheTTLs` |

## Residual (not claimed landed)

### Phase 1 leftovers

- **Landed this pack:** synthesized Responses `created_at`; Anthropic→Chat thinking on tool turns; Anthropic→Responses close-message-before-thinking; cancel upstream before closing body.
- `TestPassthroughLifecycle_LaterTurnPreOutputRateLimitRequestsReconnect` — **residual**. The freeze test is wired to the WS v2 passthrough stack (`newStagedPassthroughConn`, ingress hooks, `OpenAIWSIngressModeHTTPBridge`). Fork uses HTTP-bridge (`HTTPBridgeEnabled`) and already has later-turn replacement-account 429. Porting this named test would require wholesale WS replace, which is forbidden.

### Phase 2 remaining wiring

- Fingerprint **default off** resolver + outbound header/body rewrite + seed SQL **224** (not 225) **landed**. Profit-control stays default-off.
- Guardian affinity is applied on HTTP Responses + WS first-message context; profit-control stays default-off.

### Brandon 2026-09-10

- GPT Image 2.5 LiteLLM 单价：**不采用**（生图账单继续 gpt-image-2）。
- OpenAI Fast 写入账单 / 分组强制 Fast / 免费 Fast 按标准档扣费：**不采用**。
- 渠道时段/工作日/倍率价、分组长上下文阶梯价、dash 分组定价快照进计价器：**不采用**。
- 计价相关后续同步默认跳过，除非 Brandon 另说。

### Phase 3 B remaining

- N24-img25 GPT Image 2.5 catalog/pricing (`TestGPTImage25PricingDoesNotUseLegacyImageRates`) — **Brandon 否决改价**；fork catalog stays gpt-image-2。
- N-dash `TestAPIKeyAuthSnapshotGroupPricingRoundtrip` — **Brandon 否决改价**（`LongContextPricingEnabled` + group `ModelPricing` 会改 stored billing）。
- **Landed this pack:** Ops error-detail return-to-list keeps filters (named vitest; hop-mix/attention filter stays); usage-log upstream request id SQL **227/228** (not 232/233), `actual_cost` unchanged; compact admin account list omits group graphs and keeps quality/smart-schedule columns; GPT-6 Astra IDs on the default model table, catalog admin UI stays fork.

### Phase 4–6 C (stop-and-ask / residual, not silent adopt)

- **C-fast** stored `service_tier` / group force+free Fast — **Brandon 否决改价**
- **C-price** channel time/weekday/multiplier prices — **Brandon 否决改价**
- **C-rollup** group usage daily rollup — remap; do not steal 222/223
- **C-grok** wholesale 4.6 / xhigh / Chat→Responses vision-media / same-account capacity 429 — **landed this pack** (no official Grok price cards). Residual: admin Grok Imagine media-eligibility controls on Edit Account (would overlay a large EditAccountModal section; not wholesale-replaced).
- **C-cmv2** channel-monitor v2 — **landed this pack** (v2 API + admin settings panel + user `/monitor` V1/V2 switch; quota check_mode SQL **236** not 226; image-channel-monitor kept). Residual: live quota-fetch dispatch not wired (CN usage-window fetcher missing on this fork); popular-model seed not adopted.
- **C-plaza** model plaza — **landed this pack** (new `/model-plaza` routes + zh/en i18n; paid prices = fork display chain; anonymous sees only non-exclusive groups). Residual: freeze time-of-day / long-context stored-tier showcase columns are not adopted (billing lock).
- **C-cn** / **C-minimax** / Kimi / Zhipu / MiniMax / DeepSeek — **landed this pack** (first-class platforms, quota SQL, create-account buttons, coding-plan quota probe). Ollama Cloud still residual (usage window hang-under-CN not fully ported).
- **C-plugin** OAuth outbound plugins — **landed this pack** (default off, unsigned rejected, API-key path not hijacked; SQL 246/247 not 229/230). Residual: freeze Totp step-up on plugin admin UI not ported (fork has no step-up).
- **C-allowlist** breaking group model allowlists
- **C-fable51** claude-fable-5.1 — **landed this pack** (`claude-fable-5` kept; CLI pin 2.1.258)
- **C-simple** simple-mode grouping — **landed this pack** (basic groups visible; commercial fields stripped; composite binds rejected)
- **C-deploy** Go 1.27 / GHCR path swap — 延后
- **C-passkey** never
- **C-agent** Agent Identity last — must overlay on `chatgpt_session_token`; forbid wholesale `CreateAccountModal.vue` / deleting session refresh

### Never this campaign

- Merge `sync/main-1` into real `main`
- `git push` / production deploy
- Set VERSION to 0.2.4
- `git merge upstream/main`
- Commit historical SQL 196–220 CRLF noise
