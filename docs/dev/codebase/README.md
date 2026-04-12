# Sub2API 代码地图

> 各模块的结构化文档索引。AI 助手和开发者在修改前先阅读相关模块文档。

| 模块 | 文件 | 说明 | 最后更新 |
|------|------|------|---------|
| 账号管理 | [account.md](account.md) | 账号 CRUD、Antigravity OAuth、AI Credits、批量导入 | 2026-04-12 |
| API 网关 | [gateway.md](gateway.md) | 请求转发、负载均衡、熔断、SSE 流式 | - |
| 计费 | [billing.md](billing.md) | Token 计费、定价解析、费率乘数、缓存命中计费 | - |
| 认证 | [auth.md](auth.md) | 用户登录、JWT、OAuth SSO、2FA | - |
| 支付 | [payment.md](payment.md) | Stripe、微信、支付宝集成 | - |

## 使用说明

- **新会话开始时**：先读 CLAUDE.md，再读需要修改模块的文档
- **深入探索后**：更新或新建对应模块文件
- **模板结构**：数据模型 → 关键文件 → 核心流程 → 重要机制 → 已知陷阱
