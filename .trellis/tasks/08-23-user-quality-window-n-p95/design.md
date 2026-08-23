# Design：每用户 \(Q_u\) 窗口 N + 用户格子 P95

## Boundaries

| In | Out |
|---|---|
| `users.quality_window_n`、\(Q_u\) ingest / batch / dialog / user combined cell | \(Q_a\) 全站 N、硬关闭、\(Q_{a,u}\)、billing |

旧规格 `user-quality-last-n.md`「Do not add a second settings knob」改为：全站 N 仍是 \(Q_a\) 和继承默认；\(Q_u\) 允许每用户覆盖。不是第二颗全站 Settings。

## Data

- Migration **212**（当前 `main` max 是 211）：`users.quality_window_n INT NULL`，CHECK 省略、应用层 clamp 1–100。
- Ent `User.quality_window_n` Optional/Nillable，对齐 `display_cache_token_max_mult`。
- Redis last-N JSON 增 `override_n`（仅 \(Q_u\) 用）。缺省 = 继承。上线时库里全是 NULL，旧 key 无此字段按继承，无需回填。

## Resolve

```
ResolveUserQualityWindowN(override *int, siteN int) int
  override 有效 → ClampAccountQualityWindowN(*override)
  else → siteN（已是 ResolvedWindowN，默认 20）
```

热路径（有 last-N key）：

- `live.OverrideN != nil` → 覆盖
- 否则 → 当前全站 N（继承用户跟全站变）

Redis miss（7d TTL 后首次）：查 `users.quality_window_n`，写入 `OverrideN`（若有）再 ingest。

管理员保存覆盖/清除：写 DB + `ResizeUserLastN`（设/清 `OverrideN`、`N=resolved`、trim FIFO、Recompute）。格子立刻对。

## Ingest

`ObserveAccountCompletion` 拆开：

- \(Q_a\)：`s.windowN(ctx)`（全站）
- \(Q_u\)：`s.userWindowN(ctx, userID)`（上式）

禁止再把同一颗全站 N 灌进两个窗口。

`GetUserLastNStatsBatch` / snapshot tick：按用户 resolved N stamp `n` / `window_n` / `account_quality_window_n`（三字段仍同值，避免前端 `resolveAccountQualityWindowN` 先吃到全站 N）。投影时若 `live.N != resolved`，拷贝后 Recompute 再 ToStats，列表读路径不写 Redis。

## API

- `PUT /admin/users/:id`：`quality_window_n` 指针 + `QualityWindowNSet`（omit / null / 数字），镜像 display-cache 覆盖语义。`<1` 当清除；`>100` clamp 到 100。
- `AdminUser.quality_window_n`：覆盖或 null。
- `POST /admin/users/quality-stats/batch`：stats 带该用户生效 N（已有 `n`/`window_n`）。
- 保存后调用 maintenance `ApplyUserQualityWindowN(ctx, userID, override)`。

不新开独立 PATCH，避免第三套用户更新入口。

## UI

- `UserQualityDialog`：数字框（生效 N）+「继承全站」操作。保存走 `adminAPI.users.update`。提示：这是该用户全部账号的 last-N，不是配对 N，也不是账号质量 N。
- `AccountQualityCell`：`mode==='combined' && subject==='user'` 在 p50 下加 P95 行（复用已有 `p95ToneClass` / `formatMs`）。账号 combined 不加行。
- UsersView / 智能调度顶栏格子已是 `subject=user`。`window-n` prop 仍可作继承回落；格子优先 `stats.window_n`。
- i18n zh + en。

## Compatibility

- 未设覆盖：行为与现网相同（跟全站 N，默认 20）。
- 旧 Redis key 无 `override_n`：继承。
- 前端 `resolveAccountQualityWindowN` 不用改优先级，只要用户 stats 三字段 stamp 生效 N。
- 不改 `actual_cost` / 展示变换。

## Rollback

Revert 迁移 + 代码。`quality_window_n` 可空，回滚后旧列可留。Redis `override_n` 未知字段会被旧 decode 忽略。
