# 生产事故与日志入口

本文是生产报错 / 线上排查 / 拉日志的详细 playbook。发版、预检、`update.sh` 与发布历史以 [`DEPLOYMENT.md`](DEPLOYMENT.md) 为 SSOT。

## 入口

从 `DEPLOYMENT.md` §1.1 摘抄的生产相关行：

| 项目 | 值 |
|------|----|
| 生产服务器 | `root@172.245.247.80` |
| 本地 SSH key | `%USERPROFILE%\.ssh\id_ed25519_sub2api` / `~/.ssh/id_ed25519_sub2api` |
| Compose 目录 | `/opt/sub2api` |
| Sub2API 主服务镜像 | `ghcr.io/541968679/sub2api:latest` |
| 部署日志 | `/opt/sub2api/deploy.log` |

侧车镜像与目录仍见 `DEPLOYMENT.md` §1.1，本文不重复。

## 事故顺序

1. 读本页（以及 always-apply 压缩入口）。
2. SSH 拉日志（下一节命令，原样来自 `DEPLOYMENT.md` §1.1）。
3. 需要产品内错误记录时，查 [`codebase/ops.md`](codebase/ops.md) 的 Ops 错误 API。
4. **最后**才看业务代码。禁止先搜 `gateway_handler.go` 或先改实现。

未经用户当次明确允许，不得 push / deploy。

## 拉日志命令

以下命令从 `DEPLOYMENT.md` §1.1「部署后核对」原样摘抄，不要发明新主机或新命令。

```powershell
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose ps"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "docker inspect sub2api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}}'"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose logs --tail=120 aiclient2api"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose logs --tail=120 invokeai"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "tail -n 120 /opt/sub2api/deploy.log"
```

## 产品内错误 API

管理后台 Ops 错误查询（实现说明见 [`codebase/ops.md`](codebase/ops.md)）：

- `GET /api/v1/admin/ops/request-errors`
- `GET /api/v1/admin/ops/upstream-errors`
- `GET /api/v1/admin/ops/errors`

## 安全与禁令

- 不要把 API key、密码、token、PAT 写入文档或聊天。
- 无当次用户许可：不得 push / deploy。
- 生产主服务只允许 GHCR pull，禁止生产机构建 / `sub2api-custom:*`。
- 禁止再部署 `v0.1.232` 或 `v0.1.233`。
- 当前运行版本见 `DEPLOYMENT.md` §1.1 历史表；预检与发版步骤见 `DEPLOYMENT.md` §1.2。
