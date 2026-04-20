# Sub2API 代码地图

> 各模块的结构化文档索引。AI 助手和开发者在修改前先阅读相关模块文档。
>
> **顶层架构 / 请求流 / Wire / Settings KV / 常见任务模板 / 已知坑** 在上一层的 [../ARCHITECTURE.md](../ARCHITECTURE.md)——新会话探索代码前先读它，再按下表进入具体模块。

| 模块 | 文件 | 说明 | 最后更新 |
|------|------|------|---------|
| 账号管理 | [account.md](account.md) | 账号 CRUD、Antigravity/Gemini OAuth、AI Credits、批量导入 | 2026-04-13 |
| 模型映射 | [model-mapping.md](model-mapping.md) | 模型白名单/映射配置、默认映射、网关解析、通配符 | 2026-04-12 |
| 计费系统 | [billing.md](billing.md) | 三级定价链、费用计算、费率乘数、缓存命中计费 | 2026-04-12 |
| API 网关 | [gateway.md](gateway.md) | 请求转发、负载均衡、熔断、SSE 流式 | - |
| 认证 | [auth.md](auth.md) | 用户登录、JWT、OAuth SSO、2FA | - |
| 支付 | [payment.md](payment.md) | Stripe、微信、支付宝集成 | - |

## 使用说明

- **新会话开始时**：先读 `CLAUDE.md` → `../ARCHITECTURE.md` → 本索引 → 需要修改模块的文档
- **深入探索后**：更新或新建对应模块文件；如果有架构层面的发现（新跨切面约定、新坑）再更新 ARCHITECTURE.md
- **模板结构**：数据模型 → 关键文件 → 核心流程 → 重要机制 → 已知陷阱
