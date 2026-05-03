# 上游同步记录

> 记录每次从上游 (Wei-Shaw/sub2api) 合并更新的情况，便于追踪同步状态和解决冲突。

## 当前状态

| 项目 | 值 |
|------|-----|
| 上游仓库 | https://github.com/Wei-Shaw/sub2api |
| 上游 remote 名 | `upstream` |
| 最后同步 commit | `48912014` (chore: sync VERSION to 0.1.121) |
| 最后同步日期 | 2026-05-03 |
| 上游版本标签 | v0.1.121 |

## 同步操作步骤

```bash
# 1. 拉取上游
git fetch upstream

# 2. 查看差异
git log main..upstream/main --oneline

# 3. 合并（在 main 分支上）
git checkout main
git merge upstream/main

# 4. 解决冲突（如有），优先保留二开修改
# 5. 测试
make test

# 6. 推送
git push origin main
```

## 同步记录

### 2026-05-03 — v0.1.121 同步（v0.1.113 ~ v0.1.121，9 个版本）

- **上游版本范围**: `v0.1.113` ~ `v0.1.121`（`e534e9ba..48912014`）
- **合并策略**: 在 worktree `sync/upstream-v0.1.117` 上逐 tag merge，最后合入 main
- **合并顺序**: v0.1.113 → v0.1.114 → v0.1.115 → v0.1.116 → v0.1.117 → v0.1.118 → v0.1.119 → v0.1.120 → v0.1.121 → upstream/main

- **冲突处理**:
  - `wire_gen.go` / `wire.go`（v0.1.118/v0.1.119）: 二开的 ModelPricingHandler/PricingPageHandler/LoginPageHandler 与上游 AffiliateService/AffiliateHandler 合并
  - `AccountBulkActionsBar.vue` / `AccountsView.vue`（v0.1.120）: 上游 edit→edit-selected/edit-filtered 拆分 + 保留二开 auto-assign-proxy 按钮
  - v0.1.113 ~ v0.1.117 / v0.1.121: 零冲突或自动合并

- **合并后修复**:
  - **DefaultModels auto-include 恢复**: v0.1.120 merge 时上游重写了 `account.go:IsModelSupported()` 和 `account_handler.go:GetAvailableModels()`，丢失了二开的 `openai.IsDefaultModel()` fallback 和 `seen+merge` 逻辑，已手动恢复
  - **OpenAI 用户级模型定价修复**: 发现 OpenAI 计费路径 `calculateOpenAIRecordUsageCost` 未传 UserID 到 `CostInput`，导致用户级定价覆盖对 OpenAI 模型不生效（Anthropic/Antigravity 路径无此问题）。新增 `CostInput.UserID` 字段并在 `CalculateCostUnified` 内部 Resolve 时传递

- **重要上游变更摘要**:
  - **v0.1.113**: 支付系统 v2（手续费/移动端/退款）、Auth 身份体系（OAuth 绑定解绑）、余额/配额通知、WebSearch 仿真、License MIT→LGPL
  - **v0.1.114**: Opus 4.7 支持、prompt_cache_key 注入（OpenAI→Anthropic 路径）、KYC 阻断
  - **v0.1.115**: GPT 生图支持、Auth/支付加固、Profile 重设计、403 临时冷却逻辑、RPM 优化
  - **v0.1.116**: Channel Monitor MVP、Available Channels 聚合视图
  - **v0.1.117**: GPT-5.5 模型、Monitor 清理
  - **v0.1.118**: Claude Code 完整 mimicry、cache_control TTL 5m、Codex compact、affiliate 返利
  - **v0.1.119**: 真实 CC 客户端跳过 body mimicry（恢复 prompt caching）、affiliate 完善
  - **v0.1.120**: SetSnapshot race fix、Vertex SA、zstd 解压、account bulk edit、Fast/Flex Policy、Anthropic stream EOF failover
  - **v0.1.121**: Anthropic 缓存 TTL 注入开关、sticky session 改进、分页 localStorage

- **二开功能保留验证**:
  - 全局模型计费（GlobalModelPricingService / display_rate_multiplier / cache_transfer_ratio）✅
  - 用户级模型定价覆盖（UserModelPricingService）✅（+ OpenAI 路径 bug 修复）
  - GPT-5.5 DefaultModels auto-include ✅（恢复）
  - Antigravity 缓存修复（filterAnthropicBillingHeader / sessionIDFromMetadataUserID）✅
  - 页面内容编辑器（PricingPageHandler / LoginPageHandler）✅
  - Cache diagnostics 日志 ✅

### 2026-05-02 - v0.1.117 同步（已合入 main，见上方 v0.1.121 记录）

- **工作区/分支**: `E:\cursor project\api2sub-v117` / `sync/upstream-v0.1.117`
- **上游版本**: `v0.1.117`
- **合并提交**: `37519fcb` Merge tag `v0.1.117` into `sync/upstream-v0.1.117`
- **后续本地修复提交**:
  - `511e419b` fix(frontend): default locale and interpolation for v117
  - `64b5dff2` fix(frontend): add zh login locale keys
  - `243eae93` fix(frontend): add missing zh dashboard labels
  - `9ca7e522` fix(frontend): complete v117 zh locale coverage

- **关键处理**:
  - 将前端默认语言调整为 `zh`，避免默认进入英文界面。
  - 修复 vue-i18n 插值格式，避免充值/支付等金额变量显示异常。
  - 补齐 v117 新增/二开页面中文 locale，覆盖页面内容、登录页配置、定价页配置、模型配置、模型定价、API Key 引导、账号/用户/代理/使用记录、支付/充值/定价页等区域。
  - 补齐 `common.done` 到 en/zh，修复 API Key 引导中直接显示变量名的问题。

- **本地服务状态**:
  - 前端：`http://localhost:5180`
  - 后端：`http://localhost:18082`
  - 后端应以 `RUN_MODE=standard` 运行；`RUN_MODE=simple` 会导致管理员菜单被裁剪。

- **验证结果**:
  - `pnpm typecheck` 通过。
  - i18n key 对比：`missing zh count 0`。
  - 浏览器自动化抽查 `/pricing`、`/keys`、`/admin/model-config`、`/admin/page-content`、`/admin/users`、`/admin/accounts`、`/admin/proxies`、`/admin/usage`，未发现 raw i18n key 或 intlify missing-key 警告。
  - 管理员侧栏在 standard mode 下完整显示渠道管理、账号管理、模型配置、页面内容、订单管理、充值配置等菜单。

- **已知注意事项**:
  - 上游 `v0.1.117` tag 内 `backend/cmd/server/VERSION` 仍为 `0.1.116`，所以页面左上角显示 `v0.1.116` 是上游版本文件滞后，不代表运行错分支。
  - 如果浏览器仍显示少量菜单，优先退出重登或清理 localStorage，避免沿用 simple-mode 缓存用户态。
  - 当前记录的是独立 worktree 的合并验证进度，尚未 push，也未部署。

### 2026-04-14 - v0.1.112 同步（Cursor 兼容 + 支付/移动端修复）

- **上游 commit 范围**: `97f14b7a..e534e9ba`（17 commits）
- **合并策略**: `git merge upstream/main --no-ff`（保留 merge commit，便于回溯）
- **冲突文件**: **无**。所有上游改动文件与本地二开改动文件完全不重叠，自动合并全部成功

- **重要上游变更**:
  - **Cursor 兼容修复**（`openai_gateway_chat_completions.go` / `openai_codex_transform.go`）：
    - 兼容 Cursor `/v1/chat/completions` 传入的 Responses API body
    - Cursor raw body 透传路径剥离 Codex 不支持的 Responses API 参数
  - **Anthropic 非流式空 output 修复**（`openai_gateway_messages.go`）：
    终态事件 output 为空时从 delta 事件重建响应内容，避免空响应
  - **支付系统修复**（`payment/*`）：
    - Alipay/Wxpay direct provider 类型映射修复
    - 启用跨提供商负载均衡
    - 订单过期逻辑微调
  - **前端移动端修复**：
    - `DataTable.vue` 手机端双重渲染问题
    - `AccountUsageCell.vue` 引入 IntersectionObserver 懒加载（**注意见下**）
    - 版本下拉在手机端不再被裁剪（新增 `AppSidebar.spec.ts`）
    - 支付二维码降低纠错等级降低密度
  - **新 migration `097_fix_settings_updated_at_default.sql`**：恢复
    `settings.updated_at` 字段的默认值（之前迁移误丢）
  - VERSION bump: `0.1.111 → 0.1.112`
  - README 三语言：添加 aigocode 合作伙伴

- **合并后验证**:
  - `go build ./...` ✅
  - `go test -tags=unit ./internal/service/... ./internal/handler/... ./internal/payment/...` 全绿（76s）
  - `pnpm run typecheck` ✅
  - `pnpm run test:run`: **14 failed / 295 passed**
    - **8 失败是合并前就存在的**（用合并前的 `AccountUsageCell.vue` 跑同样是 8 failed），与本次同步无关
    - **6 新失败由上游 PR `abe42675` 引入**：该 PR 为 `AccountUsageCell.vue` 加了
      IntersectionObserver 懒加载（`hasEnteredViewport` ref），但没同步更新
      `__tests__/AccountUsageCell.spec.ts` 的 mock。jsdom 环境下观察器不会触发，
      所以组件一直处于未"进入视口"状态，断言全部失败
    - **评估**：这是上游 PR 的测试债，不影响生产行为；无需本地修复，等上游跟进
      （或者后续独立提 PR 修 mock）。本次同步不为此 block

- **本地二开改动保留情况**:
  - 全局定价覆盖修复（commit `dec95c75`）— 未被触碰 ✅
  - 代理批量导入格式扩展 — 未被触碰 ✅
  - Gemini google_one 批量 RT 导入 — 未被触碰 ✅
  - Model Config 页面（model-pricing/*）— 未被触碰 ✅
  - `docs/dev/codebase/` 二开文档 — 未被触碰 ✅

- **下次合并潜在冲突区域**: 若上游将来重构 `gateway_service.calculateTokenCost`
  或 `model_pricing_resolver` 需要重新整合本地 Bug A-C 的修复（详见
  `docs/dev/CHANGELOG_CUSTOM.md` 2026-04-14 第一条）

- **部署**: 已于 2026-04-14 部署到生产（`sub2api-custom:latest` 重建 + 健康检查通过）。部署指令：
  ```bash
  python deploy/remote_exec.py --update
  ```
  （旧的 `python deploy/remote_exec.py "/opt/sub2api/update.sh"` 在 Git Bash 环境会被 MSYS2 把 `/opt/...` 转成 Windows 路径，已弃用；详见 `deploy/remote_exec.py` docstring）

### 2026-04-12 — 初始克隆

- **上游 commit**: `97f14b7a` (Merge PR #1572 feat/payment-system-v2)
- **冲突**: 无（首次克隆）
- **备注**: 项目初始化，无二开修改

<!--
模板：

### YYYY-MM-DD — 简述

- **上游 commit 范围**: `abc1234..def5678`
- **重要上游变更**: 
  - xxx
  - xxx
- **冲突文件**:
  - `path/to/file.go` — 解决方式说明
- **合并后测试**: 通过 / 失败（说明）
- **备注**: 
-->
