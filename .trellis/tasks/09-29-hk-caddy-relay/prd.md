# Hong Kong Caddy reverse proxy

## Goal

把已开机的香港机器 `hk-relay`（`191.40.32.186`）做成大陆客户的 Caddy 反向代理入口，转发到布法罗源站 `zerocode.kaynlab.com`，海外用户和源站本身不动。

用户价值：大陆客户能稳定打开 API；源站继续跑 Sub2API，不搬数据库、不换主站。

## Background

大陆递归解析打不开 Cloudflare 权威的 `kaynlab.com`。源站在布法罗 `172.245.247.80`（内部名 `buffalo-origin`）。2026-09-29 已购买云途将军澳 `HK.TKO.C`，Debian 12，公网 IPv4 `191.40.32.186`。采购说明见 `docs/dev/HK_RELAY_BUYING_GUIDE.md`。机队与中转运维入口见 `docs/dev/SERVERS.md`、`docs/dev/HK_RELAY.md`。

## Confirmed Facts

- 内部名称 `hk-relay`。登录 `root@191.40.32.186`，密钥 `id_ed25519_yt_hk`。不要用 `id_ed25519_sub2api`。
- SSH 主机密钥（2026-09-29 本机握手）：`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/lfvm71fBLZs5oS7h8FCDHEENDd+HmGKP4ejsIv/jY`。
- 套餐 4 vCPU / 4 GB / 20 GB / 300 Mbps / 1000 GB。官方线路原文是尽力而为的大陆优化，不是 CN2 GIA。
- 这台只跑 Caddy。不跑 Sub2API、Postgres、Redis、Docker、`update.sh`。
- 反代到 `https://zerocode.kaynlab.com`，Host 仍为 `zerocode.kaynlab.com`。上游模型流量留在源站。
- 第一期入口用公网 IP：`http://191.40.32.186`。Caddy 听 80，反代到源站。Let's Encrypt 不给裸 IP 签证书；自签证书会被常见客户端拒绝。
- IP 入口只给自己做通路 / 流式 / 晚高峰测试。Authorization 头在大陆到香港这一段是明文，不把这个 URL 发给用户。
- 交给大陆用户之前仍要独立 `.com` / `.net`，主机名 `api.<新域名>`，注册商阿里云万网或 DNSPod，权威 DNS 在 DNSPod 或阿里云。不用 Cloudflare NS，不开橙云，不用 `kaynlab.com` 子域名，不用 `.cn`。
- A 记录 TTL 120 秒，指向 `191.40.32.186`。
- 有域名之后 Caddy 在香港终结 TLS。无论 IP 还是域名：`flush_interval -1`，请求体 ≥256 MB，读超时 3600 秒或关闭，HTTP/2、WebSocket。
- 海外继续用 `zerocode.kaynlab.com`。切流前不改源站 A 记录。
- 源站可选 `server.trusted_proxies: ["191.40.32.186"]`。
- 晚高峰 20:00–23:00 CST 电信 / 联通 / 移动流式通过后，才把新地址交给大陆用户。
- 未经当次允许不得 push、不得改源站 DNS、不得对大陆用户切流。
- 2026-09-29 本机对 `hk-relay:22` 和源站 `:22` 都出现过超时；`Test-NetConnection` 对香港 22 曾成功。超时不能单独当宕机。

## Locked Decisions

- 中转与源站拆开。不把 Sub2API 装到香港。
- 协议对客户端不变：仍是现有 OpenAI / Anthropic / Gemini 路径。第一期只换入口地址（IP），有域名后再换主机名。
- 第一期 `http://191.40.32.186` 做操作员测试。用户切流仍走 `https://api.<新域名>` 自动 HTTPS。
- 系统 Debian 12 + 内核 BBR。防火墙只留 22 密钥、80、443。
- 计划主机名设为 `hk-relay`。

## Requirements

- R1. `docs/dev/SERVERS.md` 与 `docs/dev/HK_RELAY.md` 用固定名称 `hk-relay` / `buffalo-origin` 记录 IP、密钥、用途；`AGENTS.md`、`PRODUCTION.md`、`DEPLOYMENT.md` 能指到这两份文档。
- R2. `hk-relay` 上 Debian 12 基线：主机名、BBR、密钥 SSH、防火墙 22/80/443。
- R3. 安装 Caddy，按锁定参数反代 `https://zerocode.kaynlab.com`，Host 不改写。第一期站点是 `http://191.40.32.186`。
- R4. 新域名 `api.<新域名>` 解析到 `191.40.32.186` 后，Caddy 自动签发 TLS。这一步不阻塞第一期 IP 测试。
- R5. 从香港对源站、以及经 `http://191.40.32.186` 做约 2 MB 上传、约 10 MB 下载，再打一条流式 `/v1/chat/completions`。晚高峰三网通过、且已有 HTTPS 域名后，再切大陆入口。
- R6. 源站 `zerocode.kaynlab.com` 继续指向 `172.245.247.80`。海外入口不改。
- R7. 需要真实客户端 IP 时，再改源站 `server.trusted_proxies`，并核对用量日志。
- R8. 回滚：停止宣传新域名即可；源站 DNS 与 compose 不因本任务回滚。
- R9. `CHANGELOG_CUSTOM.md` 记录机队文档与中转落地。

## Acceptance Criteria

- [ ] AC1. 文档里用 `hk-relay` 能查到 `191.40.32.186`、密钥文件名、用途、以及「不跑 Sub2API」。
- [ ] AC2. `ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@191.40.32.186` 能登录；密码登录关闭。
- [ ] AC3. 本机 `hostname` 为 `hk-relay`；`sysctl net.ipv4.tcp_congestion_control` 为 `bbr`。
- [ ] AC4. 第一期：对 `http://191.40.32.186/v1/models`（或等价）返回与源站同类的鉴权/成功响应。有域名后：`https://api.<新域名>/v1/models` 证书匹配新名。
- [ ] AC5. 一条流式 `chat/completions` 能从香港入口（第一期走 IP）跑完，不在 30–120 秒被代理切断。
- [ ] AC6. `zerocode.kaynlab.com` 仍解析到 `172.245.247.80`。
- [ ] AC7. 香港机器上没有 docker compose / Sub2API 进程。

## Out Of Scope

- 更换 `buffalo-origin`。
- 把 Sub2API 或数据库迁到香港。
- 修改客户端协议、`stream`、请求体。
- Cloudflare 代理。
- 本次默认 push / 改源站 DNS。
- 年付、升配、改买 DMIT。

## Open Questions

（无。Q1 已定：第一期用 IP。用户切流用的 FQDN 以后再定，不阻塞装 Caddy。）
