# 上游同步记录

> 记录每次从上游 (Wei-Shaw/sub2api) 合并更新的情况，便于追踪同步状态和解决冲突。

## 当前状态

| 项目 | 值 |
|------|-----|
| 上游仓库 | https://github.com/Wei-Shaw/sub2api |
| 上游 remote 名 | `upstream` |
| 最后同步 commit | _(初始克隆, 待填写)_ |
| 最后同步日期 | 2026-04-12 |

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
