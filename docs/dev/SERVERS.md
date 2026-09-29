# 生产机队

核对日期：2026-09-29。这是生产机器名称、地址、SSH 密钥和用途的入口。事故拉日志仍按角色走对应 playbook，不要两台混用。

| 名称 | 角色 | 公网 IPv4 | SSH | 用途 | Playbook |
|------|------|-----------|-----|------|----------|
| `buffalo-origin` | 源站 / 主服务 | `172.245.247.80` | `root@172.245.247.80`，密钥 `id_ed25519_sub2api` | 跑 Sub2API、PostgreSQL、Redis、AIClient2API、InvokeAI。对外域名 `zerocode.kaynlab.com` | [`PRODUCTION.md`](PRODUCTION.md)、[`DEPLOYMENT.md`](DEPLOYMENT.md) |
| `hk-relay` | 香港中转 | `191.40.32.186` | `root@191.40.32.186`，密钥 `id_ed25519_yt_hk` | 只跑 Caddy。大陆客户入口，反代到布法罗源站。不跑 Sub2API 和数据库 | [`HK_RELAY.md`](HK_RELAY.md) |

## 命名

内部文档、Trellis 任务、SSH `known_hosts` 备注一律用上表「名称」列。不要用厂商套餐名当主机名。

| 名称 | 厂商侧叫法 | 计划系统主机名 |
|------|------------|----------------|
| `buffalo-origin` | Buffalo / ColoCrossing VPS | 以机器上 `hostname` 为准 |
| `hk-relay` | 云途 YT.NET，套餐 `HK.TKO.C`，香港将军澳 TKO | `hk-relay`（已设置） |

## 不要混

- 生产报错、compose、`update.sh`、GHCR 镜像只对 `buffalo-origin`。
- `hk-relay` 没有 `/opt/sub2api`，不要在那台上 `docker compose` 或 `update.sh`。
- 两把 SSH 密钥不要交叉：源站用 `id_ed25519_sub2api`，香港中转用 `id_ed25519_yt_hk`。
- 换美国主站看 [`MAIN_SERVER_BUYING_GUIDE.md`](MAIN_SERVER_BUYING_GUIDE.md)。买香港中转看 [`HK_RELAY_BUYING_GUIDE.md`](HK_RELAY_BUYING_GUIDE.md)。
