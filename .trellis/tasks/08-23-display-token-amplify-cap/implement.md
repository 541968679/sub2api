# Implement：联合展示 cap（先设计，后写分配器）

工作区：占用 checkout `E:\cursor project\api2sub`。不要开隔离 worktree。本轮 **先把设计锁死**；分配器/Settings 在设计已写入且与 PRD 一致后再写。不 commit / push / deploy。

不要把 upstream-sync worktree 规则抄进本文。

## 顺序

1. 研究已完成（user 16，7 天展示层）。数字在 `research/display-token-distribution.md`。
2. **分配器 helper**（实现阶段才写）：`ApplyDisplayContextTokenCap`  
   输入：L1+L2 后的 in/cache/out + 展示价 + 联合 cap + output cap + seed  
   只做：jitter 合计 → 比例收缩 → 独立 output cap → 按价重算三分量成本。  
   **禁止** 把差额加回任何分量。0 cap = 原样返回。
3. **写路径**：`recordUsage` / `buildRecordUsageLog` 扣费前调用同一 helper，替换 `ActualCost`，打 `display_token_cap_applied` + used caps。Claude 与 OpenAI 共用。
4. **读路径**：`BuildUserVisibleUsage` 仅当 `applied=true` 时用 **行内 used caps** 重放。旧行不套。
5. **下游**：`computeSeparatedDisplayUsage` 新请求在 rate-scale 后调用同一 helper。
6. Settings KV + admin「展示层」两字段 + zh/en。加法合并，不覆盖 long-context 开关。
7. Migration = 当时 `main` max+1。Ent schema 加三列。禁止改历史 migration。
8. 文档：`billing.md` 展示节、`CHANGELOG_CUSTOM.md`、`.trellis/spec/backend/display-token-pricing.md`。

## 校验命令（实现阶段）

```powershell
go test -tags=unit ./internal/service -run "AllocateDisplayTokens|DisplayContextTokenCap|DisplayToken" -count=1
go test -tags=unit ./internal/handler/dto -run "DisplayTransform|UserVisible|DisplayRate|ContextTokenCap" -count=1
```

## 测试必须红的形状

- S=1.5M、C≈1M：in'+cache'=C，账下降，output 不因联合 cap 变大
- S=200k：两分量不动
- 比例：in:cache 与封顶前相同（允许 ±1 token 取整）
- 同一 request_id → 同一 C；不同 id 落在 92%–100%
- output cap 只砍 output
- used=0 / applied=false：读路径与上线前一致
- 默认 Settings 0：写路径不改 ActualCost

## 回滚点

helper + 写路径挂钩。Settings 回 0 停新降账。已 applied 行保持 used 值展示。

## 碰撞

`openai_long_context_billing_enabled` 只加不改。SettingsView / setting_service / CHANGELOG / billing.md / domain_constants 只做加法。
