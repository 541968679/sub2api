# 生产机队

核对日期：2026-10-21。这是生产机器名称、地址、SSH 密钥和用途的入口。事故拉日志按角色走对应 playbook，不要把温备当成写入源站。

| 名称 | 角色 | 公网 IPv4 | SSH | 用途 | Playbook |
|------|------|-----------|-----|------|----------|
| `new-origin` | 主站 / 写入 | `15.204.102.44` | `debian@15.204.102.44`，密钥 `id_ed25519_new_origin`，`sudo -n`。不要用 root 登录 | 跑 Sub2API、PostgreSQL、Redis。公网域名 `zerocode.kaynlab.com` | [`PRODUCTION.md`](PRODUCTION.md)、[`DEPLOYMENT.md`](DEPLOYMENT.md)、[`MAIN_SERVER_MIGRATION.md`](MAIN_SERVER_MIGRATION.md) |
| `buffalo-origin` | 主库温备 / 旧 IP 反代 / cyf | `172.245.247.80` | `root@172.245.247.80`，密钥 `id_ed25519_sub2api` | Postgres 只读跟随新源站，槽名 `buffalo_standby`。主站应用不启动。Caddy 把旧 IP 上的 `zerocode.kaynlab.com` 转到新源站。第二套在 `/opt/sub2api-cyf`，公网域名 `cyf.it.com` 已反代到 `127.0.0.1:8081`。回环端口还有 5433/6380。主站切回前先停这套 | [`MAIN_SERVER_MIGRATION.md`](MAIN_SERVER_MIGRATION.md) |
| `la-relay` | 洛杉矶中转，当前客户入口 | `64.83.27.87` | `root@64.83.27.87`，密钥 `id_ed25519_yt_hk`。系统主机名 `hk-relay-new` | `zerocode.kaynlab.asia` 在这里进。域名走 TCP 拼接，IP 直连由本机 Caddy 终结。不跑 Sub2API 和数据库 | [`LA_RELAY.md`](LA_RELAY.md) |
| `hk-relay` | 香港中转，回退 | `191.40.32.186` | `root@191.40.32.186`，密钥 `id_ed25519_yt_hk` | HAProxy 把 443 拼到源站 `8443`。客户域名已不指向这里。不跑 Sub2API 和数据库 | [`HK_RELAY.md`](HK_RELAY.md) |
| `lisa-hk` | 香港中转，已清空 | `154.64.246.176` | `root@154.64.246.176` 端口 `25924`，密钥 `id_ed25519_lisa_hk` | 丽萨进阶版。2026-10-14 卸掉 Caddy、证书和日志，80/443 关闭 | [`HK_RELAY.md`](HK_RELAY.md) |

## 命名

内部文档、Trellis 任务、SSH `known_hosts` 备注一律用上表「名称」列。不要用厂商套餐名当主机名。

| 名称 | 厂商侧叫法 | 计划系统主机名 |
|------|------------|----------------|
| `new-origin` | OVH，Debian 13 | `new-origin`（已设置） |
| `buffalo-origin` | Buffalo / ColoCrossing VPS | 以机器上 `hostname` 为准 |
| `la-relay` | 洛杉矶，公网 `64.83.27.87` | `hk-relay-new`（已设置） |
| `hk-relay` | 云途 YT.NET，套餐 `HK.TKO.C`，香港将军澳 TKO | `hk-relay`（已设置） |
| `lisa-hk` | 丽萨主机，香港三网直连进阶版，¥188/月 | `lisa-hk`（已设置） |

## 不要混

- 主站报错、compose、`update.sh`、GHCR 镜像只对 `new-origin`。
- `buffalo-origin` 的 `/opt/sub2api` 是主库温备。不在这台跑 `update.sh`，不启动主站 `sub2api`，不 `pg_promote`，不删除槽 `buffalo_standby`。
- `/opt/sub2api-cyf` 是另一套空库，项目名 `sub2api-cyf`。不挂温备卷 `sub2api_postgres_data`，也不由 `/opt/sub2api/update.sh` 更新。管理员邮箱是 `2741018493@qq.com`。密码只在该目录 `.env` 的 `ADMIN_PASSWORD`。`cyf.it.com` 的名称服务器是 Spaceship 的 `launch1.spaceship.net` 和 `launch2.spaceship.net`，A 记录是 `172.245.247.80`，没有 AAAA。现网 Caddy 和 failover 文件都有这个站点，证书由 Let's Encrypt 签发。failover 文件没有装成现网。
- `la-relay`、`hk-relay` 和 `lisa-hk` 都没有 `/opt/sub2api`，不要在这三台上 `docker compose` 或 `update.sh`。
- SSH 密钥不要交叉：新源站 `id_ed25519_new_origin`，布法罗 `id_ed25519_sub2api`，洛杉矶和香港中转都用 `id_ed25519_yt_hk`，丽萨候选 `id_ed25519_lisa_hk`。`lisa-hk` 的 SSH 端口是 `25924`，不是 22。
- 换美国主站的采购见 [`MAIN_SERVER_BUYING_GUIDE.md`](MAIN_SERVER_BUYING_GUIDE.md)。迁移和应急切回见 [`MAIN_SERVER_MIGRATION.md`](MAIN_SERVER_MIGRATION.md)。买香港中转看 [`HK_RELAY_BUYING_GUIDE.md`](HK_RELAY_BUYING_GUIDE.md)。
