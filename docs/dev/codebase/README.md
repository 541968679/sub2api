# Sub2API 代码地图

> 各模块的结构化文档索引。AI 助手和开发者在修改前先阅读相关模块文档。
>
> **顶层架构 / 请求流 / Wire / Settings KV / 常见任务模板 / 已知坑** 在上一层的 [../ARCHITECTURE.md](../ARCHITECTURE.md)——新会话探索代码前先读它，再按下表进入具体模块。

| 模块 | 文件 | 说明 | 最后更新 |
|------|------|------|---------|
| Grok Codex + 跨平台调研 | [../GROK_CODEX_AND_CROSS_PLATFORM_RESEARCH_2026-07-15.md](../GROK_CODEX_AND_CROSS_PLATFORM_RESEARCH_2026-07-15.md) | 需求 A：Grok Key + Codex 多轮（WS→HTTP/SSE）；需求 B：OpenAI Key 直用 Grok；与上游差异、故意不合并原因、方案选项 | 2026-07-15 |
| 账号管理 | [account.md](account.md) | 账号 CRUD、Antigravity/Gemini OAuth、AI Credits、批量导入、OpenAI Claude-GPT bridge 账号绑定 | 2026-06-09 |
| 模型映射 | [model-mapping.md](model-mapping.md) | 模型白名单/映射配置、默认映射、网关解析、通配符、Claude-GPT 账号级映射 | 2026-06-03 |
| 计费系统 | [billing.md](billing.md) | 五级定价链、费用计算、展示变换、费率乘数、缓存命中计费、Claude-GPT bridge 缓存展示 | 2026-06-03 |
| API 网关 | [gateway.md](gateway.md) | 请求转发、负载均衡、熔断、SSE 流式、Antigravity Claude-GPT bridge preflight；bridge timeout 调查见 [../OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md](../OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md)；Grok/Codex 多轮与 OpenAI Key 跨平台 Grok 见 [../GROK_CODEX_AND_CROSS_PLATFORM_RESEARCH_2026-07-15.md](../GROK_CODEX_AND_CROSS_PLATFORM_RESEARCH_2026-07-15.md) | 2026-07-15 |
| Risk Control | [risk-control.md](risk-control.md) | Content moderation, keyword/hash pre-blocking, cyber session isolation, audit records, and fork-local billing invariants | 2026-07-11 |
| Data consistency | [data-consistency.md](data-consistency.md) | Account pagination query isolation, requested-model usage aggregation, and version-aware OpenAI model-sync URLs | 2026-07-11 |
| OpenAI Image URL Relay Diagnostics | [../OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md](../OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md) | Production `gpt-image-2` URL-response behavior, native 4K channel tests, and image URL download timing splits | 2026-06-30 |
| Channel Monitor | [channel-monitor.md](channel-monitor.md) | Admin monitor CRUD, OpenAI chat/responses api_mode, request templates, checks, and rollups | 2026-06-07 |
| Image Channel Monitor | [image-channel-monitor.md](image-channel-monitor.md) | Dedicated OpenAI-compatible image generation monitor with custom API and OpenAI API-key account sources | 2026-07-10 |
| Batch Image | [batch-image.md](batch-image.md) | Gemini/Vertex asynchronous image batches, frozen-balance settlement, owner-scoped downloads, and fork-local preservation boundaries | 2026-07-11 |
| Subscription management | [subscription.md](subscription.md) | Soft-delete revoke/restore, admin history visibility, uniqueness, and cache invalidation boundaries | 2026-07-11 |
| Ops | [ops.md](ops.md) | Admin operations dashboard, lightweight realtime projections, batched group capacity, Redis slot cleanup, and availability alerting | 2026-07-11 |
| Announcements | [announcements.md](announcements.md) | Admin-authored announcements, popup scheduling, dashboard banners, and API key usage rules | 2026-06-05 |
| OpenAI Claude-GPT Bridge | [../OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md](../OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md) | OpenAI account-side Claude-GPT bridge for Antigravity groups, including routing, billing, usage, cache-display override, deployment status, and context-window notes | 2026-06-03 |
| Claude-GPT Bridge Timeout Investigation | [../OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md](../OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md) | Intermittent empty-reply/timeout evidence, official upstream comparison, 429-to-native-503 root cause, strict-routing design, count_tokens adaptation, tests, rollout, and acceptance criteria | 2026-07-10 |
| Kiro Gateway 附加项目 | [kiro-gateway.md](kiro-gateway.md) | 独立 sidecar 接入 Kiro 反代 API 到 Sub2API | 2026-05-10 |
| InvokeAI Canvas PoC | [invokeai-poc.md](invokeai-poc.md) | 独立部署 InvokeAI 画布并接入 Sub2API OpenAI-compatible 图片 API，包含 API-only 多图并行队列 | 2026-05-30 |
| 认证 | [auth.md](auth.md) | 用户登录、JWT、OAuth SSO、2FA、版本化法律确认 | 2026-06-11 |
| Public Settings Safety | [settings-security.md](settings-security.md) | Branding URL sanitization and HTML output escaping boundaries | 2026-07-11 |
| 支付 | [payment.md](payment.md) | Stripe、微信、支付宝集成 | 2026-05-29 |
| 兑换码 | [redeem.md](redeem.md) | 兑换码生成、兑换、批次限兑、用户历史 | 2026-06-27 |

| Affiliate | [affiliate.md](affiliate.md) | Invitation relationships, rebate accrual, transfer snapshots, and admin audit records | 2026-07-11 |
| Distribution | [distribution.md](distribution.md) | Distribution agent application, review, wallet, and ledger | 2026-07-11 |

## 使用说明

- **新会话开始时**：先读 `CLAUDE.md` → `../ARCHITECTURE.md` → 本索引 → 需要修改模块的文档
- **深入探索后**：更新或新建对应模块文件；如果有架构层面的发现（新跨切面约定、新坑）再更新 ARCHITECTURE.md
- **模板结构**：数据模型 → 关键文件 → 核心流程 → 重要机制 → 已知陷阱
