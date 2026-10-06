# 主站迁移方案

记下日期：2026-10-01。采购背景在 [`MAIN_SERVER_BUYING_GUIDE.md`](MAIN_SERVER_BUYING_GUIDE.md)。现网机器在 [`SERVERS.md`](SERVERS.md)。

新机器已经买下，系统已装好。公网 IP 和 SSH 见文末。机房城市还没确定，所以还没写进机队表。2026-10-01 起公网流量在这台机器上。

## 结论

接受一次最多 30 分钟的停机。不做两台同时写库。

窗口外把 Postgres 热备铺好。窗口里只做：停旧进程、等备库追上并提升、拷 Redis、启动新进程、烟测、旧 Caddy 改指新机器、再改 DNS。目标大约 10 分钟，上限 30 分钟。

用户侧：域名、API Key、后台账号、计费数据都不变。正在生成的流会被切断。停机期间新请求失败。旧 IP 上的 Caddy 改指新机器之后，DNS 还没刷新的客户端和香港中转也能回来。

## 操作系统

安装 **Debian 13.7（trixie）amd64**。这是 2026-10-01 的当前稳定版，完整安全支持到 2028-08-09。

不要装 Debian 12。Bookworm 从 2026-07-11 起只剩 LTS。香港中转那台是更早装的 Debian 12，不作为这台新源站的模板。

安装盘用网络安装镜像：

`https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-13.7.0-amd64-netinst.iso`

厂商面板里如果有模板，选 Debian 13 / 13.7 / trixie，64 位。没有就用上面的 ISO 做虚拟光驱。不要选 Ubuntu，不要选 testing / sid。

安装界面：

| 项 | 选 |
|---|---|
| 语言 | English |
| 地区时区 | UTC。用量和后台的上海时间由容器里的 `TZ=Asia/Shanghai` 负责 |
| 主机名 | `new-origin`。机房城市确定后改成机队表里的正式名字 |
| 域名 | 留空 |
| 软件 | 只勾 SSH server 和 standard system utilities |
| 磁盘 | 一块系统盘时选 Guided - use entire disk，所有文件一个分区。有两块一样的盘、面板又没做硬件 RAID1 时，选带 RAID1 的引导分区 |

不要勾桌面环境、Web server、邮件、打印、DNS。不要在安装盘里装 PostgreSQL、Redis、Nginx。Docker、Caddy、WireGuard 等系统装完再从官方源装。

这台机器走 OVH 的系统模板，不是 Debian 安装盘。模板向导只有一个「Your Public SSH key」框，不能选用户。公钥会写进默认管理员 `/home/debian/.ssh/authorized_keys`。用户名是 `debian`，带 sudo，密码另由装机邮件发出。`root` 没有密码，默认也不能直接 SSH 登录。

装完后的登录命令是 `ssh -i $HOME\.ssh\id_ed25519_new_origin debian@<新IP>`。进去之后用 `sudo -s` 进入 root。私钥只留在本机 `%USERPROFILE%\.ssh\id_ed25519_new_origin`。

OVH 选多块盘时默认做 RAID1，可用容量等于一块盘。主机名填 `new-origin`。系统选 Debian 13（模板名 `debian13_64`）。

```text
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFPYuQ2cOBmo1Mr/WZC2+goRcfHVBAAKbMgtOVTZzaQV sub2api-new-origin
```

## 现网实测（2026-10-01）

| 项 | 值 |
|---|---|
| 源站 | `buffalo-origin`，`172.245.247.80` |
| 在跑的容器 | `sub2api`（`ghcr.io/541968679/sub2api:latest`，healthy）、`sub2api-postgres`（`postgres:18-alpine`）、`sub2api-redis`（`redis:8-alpine`） |
| Postgres | 逻辑体积 15 GB，数据卷 16.74 GB，`wal_level=replica`，`max_wal_senders=10` |
| Redis | 约 8.3 MB，没有密码 |
| 应用数据卷 `sub2api_data` | 141.5 MB |
| 宿主机 Caddy | 在跑，配置 `/etc/caddy/Caddyfile`（793 字节，2026-08-12） |
| 磁盘 | 96 GB 已用 54 GB，剩余约 38 GB |
| 域名 | `zerocode.kaynlab.com` 的 A 记录指向源站，Cloudflare 灰云，前面不是代理 |
| 香港中转 | `hk-relay` `191.40.32.186` 反代主机名 `https://zerocode.kaynlab.com`。客户入口 `https://zerocode.kaynlab.asia` |

AIClient2API 和 InvokeAI 当时没有容器在跑，不在这次切换里。

## 为什么不做 0 感知双写

后台 OAuth 刷新没有跨进程锁。配额冲刷和调度 outbox 两台一起跑，可能重复记账或打乱调度。主进程收到停止信号后大约 5 秒就退出，等不完长流。一次短停、全程只有一个写入方，比双写干净。

## 搬家范围

一起走的是主服务、Postgres、Redis、宿主机 Caddy，以及 `/opt/sub2api` 的 `.env` 和数据卷里的 `config.yaml`。

`JWT_SECRET`、`TOTP_ENCRYPTION_KEY`、数据库密码原样拷贝。否则全员掉登录，开了两步验证的账号进不去。

镜像钉死切换当晚正在跑的 GHCR digest。这个窗口里不发新版本。

香港中转不搬。它继续反代原域名。

## 窗口之前

用户无感。

1. 新机器按上一节装好 Debian 13.7。装 Docker、Caddy、时间同步。防火墙公网只开 22、80、443。
2. 两台之间拉 WireGuard。Postgres 5432 和 Redis 只走这条内网。
3. 旧库建复制账号和复制槽。`pg_hba` 只允许新机器的内网地址，然后 `reload`。现网已经是 `wal_level=replica`，不用重启 Postgres。
4. 新机器用同版本 `postgres:18-alpine` 做基础备份，挂成热备，持续追 WAL。大约 16 GB 的初次拷贝放在窗口外。
5. 盯住复制槽。旧机只剩大约 38 GB 空闲。备库断线时，槽会把 WAL 堆在旧盘上。保留的 WAL 超过 5 GB 就处理。计划取消时先删槽。
6. 拷贝 `/opt/sub2api/.env`、数据卷配置、镜像 digest、宿主机 Caddyfile 和 Caddy 证书目录。域名还指着旧 IP 时，新机器做不了 HTTP 校验签发，证书要提前放好。
7. 把 `zerocode.kaynlab.com` 的 A 记录 TTL 降到 60 秒，并确认没有 AAAA。旧 TTL 是 300 秒，改完等它过期即可，不必空等 24 小时。
8. 用前一天用量挑最闲的一小时。对外仍按最多 30 分钟通知。窗口内不发版，不做改表结构的操作。

Redis 只有 8 MB。应用停掉之后再拷，不必做在线复制。

## 切换当晚

在「旧 Caddy 改指新机器」之前，任何一步失败：停掉新库，在布法罗重新启动原来的 `sub2api`。数据还在旧库上。

| 步骤 | 做什么 | 大约耗时 |
|---|---|---|
| 1 | 停布法罗上的 `sub2api`。Postgres 和 Redis 继续跑 | 10 秒 |
| 2 | 等新库回放追上，延迟为 0，然后 `pg_promote` | 通常 1 分钟内 |
| 3 | 停应用之后拷 Redis，新机器本地启动 Redis | 1 分钟内 |
| 4 | 新机器用钉死的同一镜像启动 `sub2api`，数据库和 Redis 都指本地。已有数据不会重建管理员 | 1 分钟 |
| 5 | 在新机器上查 `/health`、`/v1/models`、后台登录，打一条便宜的补全，确认 `usage_logs` 有新行 | 3 分钟 |
| 6 | 旧 Caddy 改成反代新机器的内网地址，不要反代域名本身。`caddy reload` | 几秒 |
| 7 | Cloudflare 把 A 记录改成新 IP | 几秒 |

第 6 步做完，用户侧停机结束。第 7 步之后，新的解析直接打到新机器。

提升完成之后，旧库只保留，不再启动旧的 `sub2api`，也不再让旧库接受写入。

## 回滚

烟测没过、入口还没切：丢弃新库这次提升出来的副本，在布法罗重新启动原来的 `sub2api`。

入口已经切过去、真实流量已经写进新库。布法罗留作应急热备，不退机器。

## 温备现状（2026-10-01 建立，2026-10-06 复核）

同一时间只有 `new-origin` 写库。布法罗的 Postgres 继续只读跟随，主站应用保持停止。这是快速恢复用的温备，不是每天一份 dump。

| 项 | 值 |
|---|---|
| 主库 | `new-origin` `15.204.102.44`，`pg_is_in_recovery()` 为假 |
| 备库 | `buffalo-origin` `172.245.247.80`，`pg_is_in_recovery()` 为真 |
| 复制槽 | `buffalo_standby`。主库上 `max_slot_wal_keep_size=5GB` |
| 通道 | 新机器 `pg-wg-proxy` 监听 `10.88.0.2:5432`，只转给本机 `127.0.0.1:5432`。UFW 只放行 `10.88.0.1` |
| 建立时延迟 | `replay_lag` 约 0.16 秒，`pg_stat_wal_receiver.status=streaming` |
| 2026-10-06 复核 | 接收端仍是 `streaming`，上游 `10.88.0.2:5432`。主库槽 `active=true`，`replay_lag` 约 0.08 秒 |
| 一次性拷贝 | 约 16 GB，走 WireGuard，大约十几分钟。不经过用户访问的链路 |
| 之后的流量 | 只传新增 WAL。切换后约 50 分钟的样本是每小时约 80 MB。2026-10-06 主库 `pg_stat_wal` 自 2026-10-01 12:03 +08 起写出 26 GB，约 5 GB/天 |
| 断线上限 | 槽最多多留 5 GB。按 2026-10-06 的速度大约一天。`wal_status=lost` 后要整库重做，不能拿旧目录硬提升 |
| 布法罗应用 | 保持 `exited`。override 仍钉着 2026-10-01 的 `ghcr.io/541968679/sub2api@sha256:fd11f651b5c4e150ba4e5bbabed0b5f799bd4b378ee28a7d45d9073d3ad37694`。新源站此后已继续发版，切回前要按下面的步骤对齐 digest |
| 布法罗 Redis | 仍是切换前的旧快照，应急时不能用 |
| 布法罗 Caddy | 现网把 `zerocode.kaynlab.com` 反代到 `10.88.0.2:8080`。切回文件是 `/root/sub2api-migration/Caddyfile.failover`，把该域名改回 `127.0.0.1:8080`。两份文件都有 `cyf.it.com` → `127.0.0.1:8081`。failover 仍只改 `zerocode` 的上游，cyf 保持 `127.0.0.1:8081` |
| 机器量级 | 布法罗约 6 GiB 内存，5 vCPU，根盘 96 GB。2026-10-06 清掉旧镜像并拉起 cyf 之后，已用 33 GB，剩余约 58 GB。只能应急，不能长期顶高峰，也不能和另一套满载服务同时顶主站流量 |

平时不要做：不要在布法罗启动主站 `sub2api`，不要在 `/opt/sub2api` 跑 `update.sh`，不要 `pg_promote`，不要删槽 `buffalo_standby`。布法罗上的 `pg-wg-proxy` 保持 disabled。不要执行 `/root/sub2api-migration/run-basebackup.sh`，那个脚本会先清空数据卷。`/etc/sub2api-cutover-complete` 已在 2026-10-03 为 `v0.1.306` 部署创建，不要删。新源站的 `update.sh` 只在明确要求部署时跑。

## 第二套独立站点 cyf（2026-10-06）

这套和温备无关。目录 `/opt/sub2api-cyf`，compose 项目 `sub2api-cyf`。容器是 `sub2api-cyf`、`sub2api-cyf-postgres`、`sub2api-cyf-redis`。端口只绑 `127.0.0.1:8081`、`127.0.0.1:5433`、`127.0.0.1:6380`。

应用镜像与当时新源站正在跑的 digest 相同：`ghcr.io/541968679/sub2api@sha256:9eba78c3254b77b35c46c5c9851f5c4503bbeecdf606841f0c01b198d3ce9c5e`。Postgres 卷是 `sub2api-cyf_postgres_data`，`PGDATA=/var/lib/postgresql/data`。它不是副本，`pg_is_in_recovery()` 为假。库里只有管理员 `2741018493@qq.com`。2026-10-06 把最初自动安装的邮箱改成了这一条，密码没有重设。密码和 `JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 只在该目录 `.env`，和 `/opt/sub2api/.env` 不同。不要把温备那份 `sub2api_postgres_data` 挂进来，也不要恢复主站备份。

布法罗上的标签 `ghcr.io/541968679/sub2api:latest` 仍指向温备钉住的 `sha256:fd11f651b5c4e150ba4e5bbabed0b5f799bd4b378ee28a7d45d9073d3ad37694`。cyf 按 digest 引用新镜像，没有挪动这个标签。

2026-10-06 18:03 +08 验收时，本机 `http://127.0.0.1:8081/health` 为 200，不带密钥的 `/v1/models` 为 401。温备仍是 `streaming`，槽 `buffalo_standby` 在主库上 `active=true`、`wal_status=reserved`。`zerocode.kaynlab.com` 经布法罗 `172.245.247.80` 仍是 `/health` 200、`/v1/models` 401。

2026-10-06 操作员把 `cyf.it.com` 的 A 记录指到 `172.245.247.80`，TTL 300，没有 AAAA。名称服务器仍是 `launch1.spaceship.net` 和 `launch2.spaceship.net`。`api.cyf.it.com` 仍不存在，证书签在 `cyf.it.com` 上。Let's Encrypt 证书使用者是 `cyf.it.com`，签发者 `YE1`，有效期到 2027-01-04 09:18:44 GMT。现网和 `/root/sub2api-migration/Caddyfile.failover` 都加了同一站点块，上游 `127.0.0.1:8081`，请求体 256MB，`flush_interval -1`。两份都 `caddy validate` 通过。只 reload 了现网，failover 没有装成现网。改前备份是 `/etc/caddy/Caddyfile.bak-cyf-20261006T101712Z` 和同名的 failover 备份。公网 `https://cyf.it.com/health` 为 200，不带密钥的 `/v1/models` 为 401。`zerocode.kaynlab.com` 经这台机器仍是 200 和 401，现网上游仍是 `10.88.0.2:8080`。

这台机器的磁盘和 IP 是两套共用的。数据库和密钥分开，主机故障仍然一起受影响。主站应急切回前，先在 `/opt/sub2api-cyf` 执行 `docker compose stop`。

问「温备还在不在」时，两边各看下面的 SQL。备库应是 `streaming`，主库槽应是 `active=true` 且 `wal_status` 不是 `lost`，`replay_lag` 应在 1 秒内。PG 18 的 `pg_stat_wal_receiver` 没有 `replay_lag` 和 `received_lsn`。

```bash
# 布法罗
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT pg_is_in_recovery();"
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT status, sender_host, sender_port, slot_name, last_msg_receipt_time FROM pg_stat_wal_receiver;"
# 新源站
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT slot_name, active, wal_status, pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)) AS retained FROM pg_replication_slots;"
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT application_name, state, replay_lag FROM pg_stat_replication;"
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT pg_is_in_recovery();"
```

## 应急切回

适用情况：OVH 被封、机器失联，或者公网被打到短时间修不好。布法罗在另一家厂商。SSH 到布法罗和 Cloudflare 面板都不经过 OVH。能连上 OVH 时优先在 OVH 上修，不要为了几分钟抖动就提升备库。

目标大约 15 分钟。香港中转跟着 `zerocode.kaynlab.com`，不用改中转配置。TTL 是 60 秒。布法罗是 5 vCPU 的小机器。切回去之后用户能用，高峰会比 OVH 慢。它是应急源站，不是长期源站。若第二套独立站点已经在跑，先停掉它，把 CPU 让给主站。

1. SSH：`root@172.245.247.80`，密钥 `id_ed25519_sub2api`。用上一节的 SQL 确认备库仍是 `streaming`，或至少 `pg_is_in_recovery()` 为真且最后收日志的时间还在第 4 步的保留窗口内。
2. 若 `/opt/sub2api-cyf` 的 compose 在跑，在该目录执行 `docker compose stop`，停掉应用、Postgres 和 Redis。不要在 `/opt/sub2api` 里做这一步。
3. 新源站还连得上时：先停新机器上的 `sub2api`，只停应用，Postgres 和 Redis 继续跑。等 `replay_lag` 变成 0 再提升。
4. 新源站还连得上时先看槽。`wal_status=lost` 就停止，不 `pg_promote`，改走整库重做。新源站连不上时，只用布法罗已经收到的日志：接收端仍是 `streaming`，或 `last_msg_receipt_time` 仍在保留窗口内，才提升。窗口按 WAL 约 5 GB/天、槽上限 5 GB 估算，大约一天。超出窗口就停止，最后一段没送到的写入不能靠提升找回来。
5. 在布法罗提升：`docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT pg_promote();"` 然后确认 `pg_is_in_recovery()` 变成假。
6. Redis。连得上新机器：在新机器上 `docker exec -e REDISCLI_AUTH sub2api-redis redis-cli SAVE`，把 RDB 拷进布法罗的 `sub2api_redis_data` 再启动 Redis。连不上：`docker exec -e REDISCLI_AUTH sub2api-redis redis-cli FLUSHALL`。布法罗上现有的 Redis 是 2026-10-01 04:20 UTC 的快照，不能直接给应用用。账单以 Postgres 为准。
7. 对齐应用镜像后再启动，不要跑 `update.sh`，不要使用 `weishaw/sub2api:latest`。新源站可达时读取 `docker inspect sub2api --format '{{.Image}}'`。GHCR 也可达时，把该 digest 写进布法罗 `/opt/sub2api/docker-compose.override.yml` 的 `sub2api.image`，仓库必须是 `ghcr.io/541968679/sub2api`。GHCR 不可达时，用 override 里现有的 `sha256:fd11f651b5c4e150ba4e5bbabed0b5f799bd4b378ee28a7d45d9073d3ad37694` 启动，并记下这是版本回退。然后只执行 `cd /opt/sub2api && docker compose up -d sub2api`。不要无服务名的 `docker compose up -d`，那会把 AIClient2API 和 InvokeAI 也拉起来。
8. 本机烟测：`http://127.0.0.1:8080/health` 应返回 ok，不带 Key 的 `/v1/models` 应返回 401。
9. 切换入口：`caddy validate --config /root/sub2api-migration/Caddyfile.failover` 通过后，再 `cp` 到 `/etc/caddy/Caddyfile` 并 `systemctl reload caddy`。这把 `zerocode.kaynlab.com` 改回 `127.0.0.1:8080`。2026-10-06 两份文件都已经有 `cyf.it.com`，上游是 `127.0.0.1:8081`。整份覆盖前先确认 failover 里这块还在，否则这次复制会把 cyf 的入口丢掉。
10. Cloudflare 把 `zerocode.kaynlab.com` 的 A 记录改回 `172.245.247.80`，TTL 60，灰色云朵，不要加 AAAA。

提升之后不要再启动新机器上的 `sub2api`。两台同时写会把额度和账单记重。OVH 以后如果回来，先保持应用停止，再把它做成新的只读备库。

## 切完要看的

- 新容器 healthy，镜像 digest 与切换前一致。
- `https://zerocode.kaynlab.com/health` 和 `/v1/models` 在新 IP 上正常。
- 旧 IP 的 HTTPS 转到新服务。
- `https://zerocode.kaynlab.asia/v1/models` 在没带 Key 时返回 401，而不是 502。
- 后台能登录，一条真实请求写入用量。
- 接下来 1 小时看 5xx、路由 503、支付回调。

## 新机器（2026-10-01，尚未接流量）

| 项 | 值 |
|---|---|
| 名称 | `new-origin`。2026-10-06 起列入 [`SERVERS.md`](SERVERS.md) |
| 公网 IPv4 | `15.204.102.44` |
| SSH | `debian@15.204.102.44`，密钥 `id_ed25519_new_origin`。`sudo -n` 可用。不要用 root 登录 |
| 系统 | Debian GNU/Linux 13 (trixie)，内核 `6.12.111+deb13-amd64`，时区 UTC |
| 硬件 | AMD Ryzen 9 5900X（12 核 24 线程），62 GiB 内存，两块 476.9 GB NVMe 做 RAID1。根分区约 467 GB，准备完成时已用 19 GB |
| 状态 | 2026-10-01 起这台机器是源站。公网 A 记录是 `15.204.102.44`。布法罗只保留旧 IP 反代 |

## 窗口之前的进度（2026-10-01 04:03 UTC）

已完成，用户无感：

1. Docker 29、Compose v5.5.1、chrony、UFW 已装。公网只放行 22、80、443。Caddy 用官方二进制 v2.11.4，不走已过期的 Cloudsmith 源。
2. WireGuard 已通。布法罗 `10.88.0.1/24`，新机器 `10.88.0.2/24`，UDP 51820 只对对方公网 IP 开放。握手正常，延迟约 82 ms。私钥只在各自 `/etc/wireguard/privatekey` 和 `wg0.conf`，模式 600。
3. 复制口令文件在两台机器的 `/root/sub2api-migration/`，模式 600。不要把口令写进文档或命令行。布法罗上 `replicator` 只有复制权限。`max_slot_wal_keep_size=5GB` 已 reload，没有重启 Postgres。
4. Postgres 不直接听 WireGuard。`pg-wg-proxy.service` 把 `10.88.0.1:5432` 转到本机 `127.0.0.1:5432`。Docker 代理之后，库看到的客户端地址是 `172.18.0.1`，不是 `10.88.0.2`。真正命中的 `pg_hba` 行是 `host replication replicator 172.18.0.0/16 scram-sha-256`。5432 没有绑到公网网卡。
5. 热备已在追日志。槽名 `new_origin`。新机器只启动了 `sub2api-postgres`，`pg_is_in_recovery()` 为真，收到的 LSN 和回放的 LSN 一致。布法罗上该连接是 `streaming`，延迟约 0.16 秒。当时库里有 475 个用户、4,543,004 条 `usage_logs`。没有执行 `pg_promote`。
6. `/opt/sub2api/.env` 已原样拷贝，模式 600，和布法罗的校验一致。应用数据卷 `sub2api_sub2api_data` 已拷贝（约 140 MB，含 `config.yaml`）。新机器的 compose override 钉死这三枚 digest，布法罗的 override 仍是 `:latest`：
   - `ghcr.io/541968679/sub2api@sha256:fd11f651b5c4e150ba4e5bbabed0b5f799bd4b378ee28a7d45d9073d3ad37694`
   - `postgres@sha256:4da1a4828be12604092fa55311276f08f9224a74a62dcb4708bd7439e2a03911`
   - `redis@sha256:81b6f81d6a6c5b9019231a2e8eb10085e3a139a34f833dcc965a8a959b040b72`
7. 新机器的 `update.sh` 在出现 `/etc/sub2api-cutover-complete` 之前会直接拒绝。这个窗口里不要在新机器上跑它。
8. Caddyfile 和 `/var/lib/caddy` 的证书库已拷贝，`caddy validate` 通过。服务保持 disabled，没有监听 80/443。`zerocode.kaynlab.com` 证书到期日是 2026-11-06。大约 2026-10-07 起 Caddy 会想续期，而 DNS 仍指向布法罗，所以现在不要在新机器上启动 Caddy。若切换晚于 2026-10-07，先在布法罗完成续期，再把证书库重新拷过来。

## 2026-10-01 12:25 CST 已切开

应用已经在 `new-origin` 上写库。布法罗的 `sub2api` 保持停止。旧库的复制槽 `new_origin` 已删除，避免 WAL 堆积。布法罗 Postgres 和 Redis 还在，但不再接应用写入。

已核对：

- 新容器镜像仍是 `sha256:fd11f651b5c4e150ba4e5bbabed0b5f799bd4b378ee28a7d45d9073d3ad37694`，状态 healthy。
- 提升后 `pg_is_in_recovery()` 为假。
- 本机 `/health` 返回 `{"status":"ok"}`，不带 Key 的 `/v1/models` 返回 401。
- 一条 `deepseek-v4.1-flash` 补全返回 200，并写入 `usage_logs`。
- 布法罗 Caddy 改为反代 `10.88.0.2:8080`，`https://zerocode.kaynlab.com/health` 正常。
- `https://zerocode.kaynlab.asia/v1/models` 不带 Key 返回 401。
- 新机器 Caddy 已启动。用 `--resolve` 打 `15.204.102.44` 的 `/health` 正常。

2026-10-01 已把 Cloudflare 的 A 记录改为 `15.204.102.44`，TTL 60。权威 DNS `zod.ns.cloudflare.com` 和 `audrey.ns.cloudflare.com` 都已核对。没有 AAAA。云朵仍是灰色。

短时间故障在新机器上修。OVH 被封或失联时按上面的「应急切回」提升布法罗，不要直接启动切换前那份旧库。

## DNS

2026-10-01 已在 Cloudflare 把 `zerocode.kaynlab.com` 的 A 记录改为 `15.204.102.44`，TTL 60。权威 DNS `zod.ns.cloudflare.com` 和 `audrey.ns.cloudflare.com` 都核对过。没有 AAAA。云朵是灰色。应急切回时再改回 `172.245.247.80`。

用量样本里最闲的是上海时间 05:00–07:00，最忙的是 23:00–00:00。对外仍按最多 30 分钟通知。旧 TTL 过期后就可以切，不必再等 24 小时。

## 切换当晚不要踩的坑

- 新机器上只把已经在跑的 Postgres 从热备提升，不要再做一次 `pg_basebackup`，也不要让 compose 重新 initdb。
- Redis 数据等 `sub2api` 停掉之后再拷进 `sub2api_redis_data`。
- 启动应用前再确认镜像 digest 仍是上面那三枚。布法罗若在这之前发过版，先停下来重核，不要带着新 digest 切。
- 烟测通过后再 `systemctl enable --now caddy`。在此之前 Caddy 保持停止。
- 回滚只在入口还没切、新库还没有真实写入时做。提升之后若已经有用户写入，当晚不切回旧库。
