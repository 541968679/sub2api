# 上游同步记录

> 记录每次从上游 (Wei-Shaw/sub2api) 合并更新的情况，便于追踪同步状态和解决冲突。

## 当前状态

| 项目 | 值 |
|------|-----|
| 上游仓库 | https://github.com/Wei-Shaw/sub2api |
| 上游 remote 名 | `upstream` |
| 最后同步 commit | `e534e9ba` (chore: sync VERSION to 0.1.112) |
| 最后同步日期 | 2026-04-14 |
| 上游版本标签 | v0.1.112 |

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

### 2026-04-14 — v0.1.112 同步（Cursor 兼容 + 支付/移动端修复）

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
