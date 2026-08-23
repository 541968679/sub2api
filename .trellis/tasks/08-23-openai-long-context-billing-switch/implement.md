# Implement: OpenAI 长上下文计费开关

Ordinary product work on this occupied checkout. Do not copy upstream-sync worktree rules here.

## Order

1. Backend constants + SystemSettings parse/default/update + cached reader.
2. Inject SettingService into BillingService; gate `shouldApplySessionLongContextPricing`.
3. Admin DTO GET/PUT + change log field + Wire `ProvideBillingService`.
4. Admin UI Toggle + zh/en i18n. Not public settings.
5. Tests: billing on/off/missing + setting default-on; keep existing GPT-5.4 tests.
6. `docs/dev/codebase/billing.md` + `docs/dev/CHANGELOG_CUSTOM.md`.

## Checklist

- [ ] `SettingKeyOpenAILongContextBillingEnabled = "openai_long_context_billing_enabled"`
- [ ] defaults `"true"`; parse `!= "false"`
- [ ] `IsOpenAILongContextBillingEnabled` cached; nil/missing/invalid → true
- [ ] `refreshCachedSettings` 立即刷新
- [ ] `ProvideBillingService` 注入；`NewBillingService` 签名不变
- [ ] gate 在 `shouldApplySessionLongContextPricing`
- [ ] admin DTO / handler 三处映射（GET/PUT/updated）+ dirty field
- [ ] Features Toggle + i18n
- [ ] 不改 `PublicSettings` / `GetPublicSettings`
- [ ] billing 单测 AC1–AC5；setting 单测 default-on / false
- [ ] changelog + billing.md

## Validation

```powershell
go test -tags=unit ./internal/service -run "OpenAIGPT54LongContext|OpenAILongContextBilling|ChannelIntervalsDoNotSetLongContext" -count=1
go test -tags=unit ./internal/handler/admin ./internal/handler/dto -count=1 -timeout 120s
```

Frontend：对照 Settings 表单编译（`pnpm --dir frontend run typecheck` 若时间允许）。无浏览器依赖的新交互，Toggle 是既有组件。

## Risky files

- `billing_service.go`：只加开关闸，不要改阈值公式或区间短路。
- `setting_handler.go`：三处 DTO 映射漏一个会导致 GET 有、PUT 丢。
- `SettingsView.vue`：只加 Features 卡 + form default + save field。
- 不要改 pool-sticky 脏文件。

## Do not

- git commit / push / deploy
- Gemini long-context
- threshold/multiplier editors
- public settings
- isolation worktree
