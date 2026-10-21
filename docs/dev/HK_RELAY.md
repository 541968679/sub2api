# 香港中转 `hk-relay`

核对日期：2026-10-16。客户入口在这台将军澳机器上。2026-10 的超时、断流和写挂起优化见「2026-10 优化全过程」。采购结论在 [`HK_RELAY_BUYING_GUIDE.md`](HK_RELAY_BUYING_GUIDE.md)。机队总表在 [`SERVERS.md`](SERVERS.md)。

## 名称

| 项 | 值 |
|----|----|
| 内部名称 | `hk-relay` |
| 中文用途名 | 香港中转 |
| 厂商 | 云途网络 YT.NET，控制台 [cloud.yt.net](https://cloud.yt.net/server/hk-tko) |
| 套餐 | `HK.TKO.C`（4 vCPU / 4 GB / 20 GB SSD / 300 Mbps / 1000 GB / 月付 ¥45） |
| 机房 | 香港将军澳（TKO） |
| 线路 | 官方写「尽力而为的大陆优化网络」，AMD EPYC 7K62。不要写成 CN2 GIA |
| 公网 IPv4 | `191.40.32.186` |
| 登录 | `root@191.40.32.186` |
| 本地 SSH 密钥 | `%USERPROFILE%\.ssh\id_ed25519_yt_hk` / `~/.ssh/id_ed25519_yt_hk` |
| 系统 | Debian 12 (bookworm)，sshd `OpenSSH_9.2p1 Debian-2+deb12u7` |
| 厂商主机名 | `vm20403-cur20297`（cloud-init 原名） |
| 系统主机名 | `hk-relay`（已 `hostnamectl` + `preserve_hostname: true`） |
| 入口 | HAProxy TCP 拼接，`/etc/haproxy/haproxy.cfg`（仓库副本 [`hk-relay/haproxy.cfg`](hk-relay/haproxy.cfg)）。Caddy `2.11.7` 仍安装，服务已 disable，`/etc/caddy/Caddyfile` 只用于回退 |
| SSH 主机密钥 | `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/lfvm71fBLZs5oS7h8FCDHEENDd+HmGKP4ejsIv/jY`（2026-09-29 本机首次握手写入 `known_hosts`） |

```powershell
ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@191.40.32.186
```

## 用途

大陆客户经常打不开布法罗源站 `zerocode.kaynlab.com`（权威 DNS 在 Cloudflare）。这台机器只做 TCP 拼接，不解析 HTTP：

```text
大陆客户端 → hk-relay:443（HAProxy，字节原样转发，PROXY v2）
          → new-origin:8443（Caddy 解开 PROXY，终结 TLS）
          → 127.0.0.1:8080
```

主站应用在 `new-origin`。`buffalo-origin` 只是温备，不要到那台拉主站日志或跑 `update.sh`。海外用户继续用 `zerocode.kaynlab.com`。

## 明确不做什么

- 不安装、不运行 Sub2API、PostgreSQL、Redis、AIClient2API、InvokeAI。
- 不跑现在这套 Docker Compose，不用 `update.sh`，不 pull GHCR。
- 不把 `kaynlab.com` 的 NS 改到这台机器上。
- 不在切流前把 `zerocode.kaynlab.com` 的 A 记录改到香港。
- 前面不套 Cloudflare。

## 2026-10-21 TCP 透传（当前入口）

香港不再解析 HTTP。`https://zerocode.kaynlab.asia` 仍指向 `191.40.32.186`。HAProxy 在 443 上做 TCP 拼接，给源站发 PROXY v2。源站 Caddy 在 `8443` 上解开 PROXY、做 TLS，再反代到 `127.0.0.1:8080`。客户的 HTTP/2 或 HTTP/1.1 原样到达源站，和直连 `zerocode.kaynlab.com` 同一层处理。

| 项 | 值 |
|----|----|
| 香港进程 | HAProxy，`/etc/haproxy/haproxy.cfg`，仓库副本 [`hk-relay/haproxy.cfg`](hk-relay/haproxy.cfg)。`systemctl is-active haproxy` |
| 上游 | `15.204.102.44:8443`，`send-proxy-v2`。连接、客户端、服务端、隧道超时都是 3600 秒 |
| 源站监听 | 只给香港。ufw：`8443/tcp` 来自 `191.40.32.186`。Caddy `servers :8443` 要求 PROXY，`allow 191.40.32.186/32`，`protocols h1`。不提供 HTTP/2，避免一个客户的全部请求挤在一条 TCP 上 |
| 证书 | `/etc/caddy/certs/zerocode.kaynlab.asia.crt` 与同名 `.key`。内容是香港 Caddy 在 2026-10-21 的 Let's Encrypt 证书，CN `zerocode.kaynlab.asia`，到期 2026-12-29 01:07 GMT。这是静态文件，源站不会自动续期 |
| 客户 IP | PROXY 里的地址写到 `X-Real-IP`，用量 `ip_address` 记这个地址 |
| 入口主机 | 客户自己的 `Host: zerocode.kaynlab.asia`。8443 站点同时写 `X-Sub2-Entry-Via: hk-relay` |
| 访问日志 | 源站 `/var/log/caddy/asia-relay.log`。香港不再写 HTTP 访问日志 |
| Caddy | 软件还在，`/etc/caddy/Caddyfile` 还是 2026-10-16 的 HTTP 反代配置，systemd 已 disable。不要 `start caddy`，它会和 HAProxy 抢 443 |

80 端口不再监听。以前给操作员的 `http://191.40.32.186` 也停了。客户只用 HTTPS。

回退：`systemctl stop haproxy && systemctl enable --now caddy`。源站 8443 可以留着，它不影响 443 上的直连。

续期要在 2026-12-29 之前做。把新证书放到上面两个文件，然后 `systemctl reload caddy`。不要为了续期把 443 切回香港 Caddy，除非同时停掉 HAProxy。

下面「2026-10 优化全过程」是 HTTP 反代时期的记录，只在回退到 Caddy 时才再生效。

## 2026-10 优化全过程

这份记录只覆盖将军澳 `hk-relay` 上已经落地、并且验证过的改动。目的是下次看到「耗时比网关长」或「流被切断」时，按这里的顺序核对，不要把已经否掉的办法再做一遍。仓库里的 [`hk-relay/Caddyfile`](hk-relay/Caddyfile) 是回退配置，不是现在的入口。当前入口见上面的 TCP 透传。不要在这台跑 `docker compose` 或 `update.sh`。

### 当时生效的配置（2026-10-16 18:33 +08，2026-10-21 起已停用）

| 项 | 值 |
|----|----|
| 版本 | Caddy `2.11.7`，`h1:yj0Y4fYZGPkSvibBJ1sTWE33xC0fxztVyXEW5iIdUT4=`。当天 GitHub 稳定版最新就是 `2.11.7`（2026-10-03）。没有更新的稳定版可升 |
| 对客户 | `servers.protocols h1`。ALPN 不提供 `h2` |
| 回源 | `transport http { versions 1.1 }`，`compression off`，`flush_interval -1`，`dial_timeout 30s`，`keepalive 120s` |
| 服务器超时 | `read_body 3600s`，`read_header 60s`，`write 3600s`，`idle 3600s`，`write_idle 60s`，`write_max_chunk 16KiB` |
| Host | `header_up Host zerocode.kaynlab.com`，`X-Forwarded-Host` 同样。另外写 `X-Sub2-Entry-Host {http.request.host}`，并删掉入站的 `X-Sub2-Entry-Via` |
| 访问日志 | `/var/log/caddy/hk-relay.log`，JSON。`x_request_id` 取响应头 `X-Request-Id` |
| 健康检查 | reload 后本机和中转上 `https://zerocode.kaynlab.asia/health` 都是 `{"status":"ok"}` |

备份：

| 文件 | 内容 |
|------|------|
| `/etc/caddy/Caddyfile.bak-before-2.11.7` | 升级到 2.11.7 之前 |
| `/etc/caddy/Caddyfile.bak-entry-host-20261016` | 加入口主机头之前 |
| `/etc/caddy/Caddyfile.bak-write-idle-20261016` | `write_idle 15s` 之前 |
| `/etc/caddy/Caddyfile.bak-write-idle-60s-20261016` | 改成 60 秒之前（当时是 15 秒） |

`caddy validate` 会警告第 7 行 Caddyfile 未格式化。不要顺手 `caddy fmt --overwrite`。

### 路径和四段时间

```text
客户或探测 → TokenHub（nginx/1.18.0，后面是 New API）
          → hk-relay（191.40.32.186）
          → 源站 Caddy → Sub2API
```

TokenHub 公网是 `https://tokenhub.njsrd.com/`。2026-10-16 响应头 `X-New-Api-Version: c0a0f95-18`，`/api/status` 的 `version` 相同。进程 `start_time` `1791621985`，即 2026-10-10 16:46 +08。打到香港的出口地址是 `218.93.211.227`，User-Agent 是 `Go-http-client/1.1`。用量里的用户是 `users.id` 644。`usage_logs.ip_address` 记的是客户 IP，不能用来证明请求没经过香港。

香港访问日志的时间戳是这条响应写完的时刻，不是开始时刻。网关 `duration_ms` / `first_token_ms` 是源站自己的生成时间。四段要分开算：

| 段 | 算法 |
|----|------|
| 前导 | 香港开始 − 网关或客户端开始 |
| 生成 | 网关 `duration_ms`，首字是 `first_token_ms` |
| 香港整段 | 访问日志时间戳往前推日志里的 duration |
| 尾部 | 客户端或香港结束 − 用量 `created_at`，或客户端结束 − 香港结束 |

前导加尾部约等于香港 duration 减网关 duration。用量行是转发结束后异步写的，尾部为正时只是下限。香港 `x_request_id` 拼到用量 `request_id` 上时前缀是 `client:`。

TokenHub 页面上的「首字」是他们自己收到上游第一段数据的时间，「耗时」是他们整段 `use_time`。耗时曾经跟香港 duration 对齐（那条 300 秒挂起）；后来也有行在香港已经写完之后又多记了若干秒。页面上的完成 token 大约是我们计费输出的 1.2 倍。不要用页面上的百分秒去证明香港路径很快。

业务日界按 Asia/Shanghai。日志和 `journalctl` 的纪元是 UTC。`1792108800` = 2026-10-16 00:00 UTC = 08:00 +08。在源站 Postgres 里对浮点用 `numeric`，不要 `round(float, int)`，这个函数签名不存在。从 PowerShell 用 SSH heredoc 时，行尾 CR 会让远端 bash 把结束标记看成别的名字；脚本用 base64 传，或先去掉 CR。

### 已经做过、必须留着的四步

**1. 回源只走 HTTP/1.1（2026-10-14，先在丽萨验证，再改将军澳）**

`versions 1.1 2` 时，HTTP/2 把所有对话复用到一条连接。成都移动的新 `/v1/chat/completions` 要等几十秒才有响应头，用量里的首字仍是几秒，`/health` 仍小于 2 秒。改成 `versions 1.1` 之后，短的 `kimi-k3-fast` 经香港首字 3.7 秒和 6.0 秒，直连美国 2.7 秒和 3.4 秒。不要把回源 `versions` 加回 `2`。

**2. 对客户也只提供 HTTP/1.1（2026-10-14）**

`njsrd` 在一条 HTTP/2 连接上同时上传多条 0.5–2MB 的提示词。Caddy 把这些请求压了 4–8 分钟，不同大小的请求体在同一秒才到达应用。同一 IP 的 194 字节请求大约 3 秒就结束。`servers.protocols h1` 之后，这些上传各自一条连接。模型仍然要等整段提示词到齐才出首字。不要重新打开入站 `h2`。

**3. 立即刷出（一直开着）**

`flush_interval -1`，回源 `compression off`。源站 Caddy 对 `text/event-stream` 也不再做 zstd/gzip（2026-10-16，备份 `/etc/caddy/Caddyfile.bak-sse-encode-20261016`，那是 `new-origin` 的文件，不是香港这台）。压缩和默认缓冲会把 SSE 攒到一块才发出。不要在香港或源站把事件流重新压回去。

**4. 写挂起用 `write_idle`，总写预算仍是 3600 秒（2026-10-16）**

见下一节。不要把 `write 3600s` 或传输层 `write_timeout 3600s` 改短来「修断流」。那是整段响应的硬上限，不是空闲检测。

应用层给客户 SSE 加的 `Connection: close` 到不了香港的客户侧。这是逐跳头，Caddy 会去掉。不要再加一层同样的头，也不要靠它收尾部。OpenAI 同步等待超时保持 0/0，不要为这件事改等待超时。这次挂起也不要靠新镜像解决；当时主站已是 `ghcr.io/541968679/sub2api:0.1.326`。不要重装 `v0.1.232` 或 `v0.1.233`。

### 写挂起：现象、错误改法、最终值

一条响应已经生成完，对端不再从套接字读，内核接收窗口会满。窗口满只堵住发送方的写，不会发 FIN。Caddy 2.11.4 没有「写没有进展就断开」的服务器选项，于是访问日志里的 duration 会一直算到对端自己关闭。状态 200 且 size 是整段，表示字节最终写出去了，不表示中间没有堵住。

2026-10-16 那条 TokenHub 用量（112/308，首字 5 分 2 秒，耗时 5 分 21 秒）是我们自己的 4KB 接收缓冲探测，不是客户的正常终端。探测端在响应头之前就被断开，0 字节。网关生成大约 11.8 秒，首字大约 3.2 秒。香港 duration 301.923 秒，状态 200，size 59446。300 秒对得上 New API 默认 `STREAMING_TIMEOUT`（300，环境变量可改）。他们的流式循环在写给自己的客户被堵住时停止读上游，但不上游关闭，直到这个计时。同一次另外三条探测（32KB、暂停、正常读）没有把窗口压满，香港比网关多出来不到大约 1 秒。

用户空间不读，不等于窗口已经是 0。内核接收缓冲还在代为 ACK。所以「把应用暂停一下」经常压不出窗口 0；4KB 的 `SO_RCVBUF` 才压出来了。

错误改法，不要重复：

| 尝试 | 结果 |
|------|------|
| 在站点块里写 `timeouts { write_timeout ... }` | 2.11.4 报 `unrecognized directive: timeouts`。`write_idle` 只在服务器块，且要 2.11.7 及以上 |
| 装 2.11.6 | 默认 1 分钟 `write_idle` 有两个错误：HTTP/1.1 上，带正文的 POST 流在读完正文 60 秒被切（#8103）；HTTP/2 上，两次写出之间的空隙也被算进空闲（#8118、#8119）。2.11.7 才改成只计算卡住的那一次写 |
| `apt` 不先 `apt-get update` | 候选仍是 2.11.4，升不上去 |
| 升级时让 systemd `restart` | 2026-10-16 16:25 +08，旧进程 `stop-sigterm` 超时后被 SIGKILL。当时正在进行的流断了一次。日志里大约 `1792139125` 开始提供 2.11.7，`1792139139` 又 reload 了一次 |
| 用 nginx 或 Envoy 换掉 Caddy | 不需要。`write_idle` 就是 Caddy 自己的旋钮，语义等同 nginx 的 `send_timeout`：写没有进展才计时 |

升级顺序：先 `apt-get update`，确认候选是 2.11.7 而不是 2.11.6；保留旧 Caddyfile（2.11.4 的文件在 2.11.7 上仍然合法）；装包若必须停进程，接受那一次在途流被杀掉；之后的超时调整只用 reload。

2.11.7 的 `write_idle`：一次底层写在 `write_max_chunk`（这里 16KiB）范围内完全没有进展，就断开。这块写完，计时清零。两次写之间的空隙不计。官方默认就是 1 分钟。源码注释写明：大响应和 SSE 只要每次写还有进展，就不受影响。

当天先设成 15 秒，是为了把 300 秒的死写收短。16:25 之后到约 18:04，`journalctl -u caddy` 里 `aborting with incomplete response` 且错误为 `writing: ... i/o timeout` 的有 8 条，全部在升级之后，第一条约 16:48 +08（`1792140505.1`）。其中 7 条对端是 `218.93.211.227`，都是 `POST /v1/chat/completions`。5 条是大约 0.5–1.4MB 的请求，整段 18–33 秒；另外两条整段只有 3.67 秒和 5.16 秒，对不上「空闲满 15 秒」这一种切法。`Accept` 是 `application/json` 或 `*/*`。New API 把终端客户的 `Accept` 原样抄到上游，只有流式且客户没带 `Accept` 时才补 `text/event-stream`。所以这个头不能当成「非流式」或「没走 TokenHub」。

同一天 12:00 +08 之后，未完成响应一共 1901 条，其中 `reading: context canceled` 1881 条，1499 条在 16:20 之前。15:00 那一小时 834 条里有 714 条来自 `118.178.84.95`，不是 TokenHub。TokenHub 这个 IP 在 15:00 的取消和 broken pipe 发生在 `write_idle` 生效之前。不要把这波取消算到 15 秒超时上。`reading: context canceled` 的方向是：香港还在读上游时，TokenHub 把请求取消了。`writing: i/o timeout` 的方向是：香港往 TokenHub 写响应，写了 15 秒没有进展，连接还在。对方若已关闭，日志是 `broken pipe`、`connection reset` 或上面的 context canceled。

2026-10-16 18:33:56 +08 把 `write_idle` 改成 60 秒并 reload（纪元 `1792146836`）。reload 日志是 `servers shutting down with eternal grace period`，在途连接没有被 SIGKILL。服务保持 active，健康检查 ok。

60 秒不会加到已经在正常写完的请求上。用同一把客户 key、经过 TokenHub、并且探测端一直读到结束的请求，香港比网关多出来的是零点几秒到几秒。会让 TokenHub「耗时」比网关长出大约 60 秒的，只有写已经卡死且没有恢复的那一类：我们要等满 60 秒才判定它不会恢复，他们的计时要算到连接结束。15 秒和 60 秒是同一根旋钮。卡死后的头几秒里，Caddy 分不出「过一会儿还会读」和「再也不会读」。没有第三个参数能让活着的写多等、死掉的写立刻断。

他们进程里才能把两边同时照顾到：客户那一头写不出去时，关掉通往香港的连接，耗时就停在客户停下的时刻。公开仓库当前的 `StreamScannerHandler` 对单次写给客户有 30 秒上限，到点会关上游。`c0a0f95` 在 `QuantumNous/new-api` 里不存在（GitHub API 422）。官方构建的版本号是 `git describe --tags`，形状像 `v1.0.0-rc.44`，不是 `c0a0f95-18`。`c0a0f95-18` 是他们编译时写进 `VERSION`、再由 `-X common.Version` 打进二进制的私有构建。前端仍链到 `https://github.com/QuantumNous/new-api`，控制台文案对得上上游 `web/src`。301 秒的挂起说明这套二进制没有在客户写卡住时关上游，而是等到默认 300 秒流式计时。不要把本机 `E:\cursor project\new-api` 当成他们的二进制。非流式的公开代码是先 `io.ReadAll` 读完上游再写给客户；那种路径里，客户读得慢不会堵住香港的写。流式路径才会。

不要把「客户侧断了」全部写成「终端没有继续读，所以香港的写停住」。后半句只被 `writing: i/o timeout` 支持，而且停住的读可能在他们的 Go 和 nginx 之间，我们没有客户侧套接字。前半句的大多数是 `reading: context canceled`。

### 复测时不要换掉流量形状

客户真实的 `kimi-k3`（用户 644，2026-10-13 00:00 UTC 到 2026-10-16 06:00 UTC，探测开始之前）大约 5.6 万条。输入中位约 99，p90 约 3700，p99 约 11 万；大请求经常是 3–5 万的缓存前缀加上几千新 token。用两句话、`max_tokens` 48 打出来的样本对不上这个分布。大请求复测用大约 6 万汉字、并发 5。TokenHub 预扣大约 ¥0.49 一条，余额不够时后继请求是 403 `insufficient_user_quota`。2026-10-16 那次 200 条里只有 17 条 200，183 条是余额不足。余额补上之前不要再打这一档。不要把 key 写进仓库、文档或聊天。

探测端要一直读到结束，超时要盖住生成时间（那次是 420 秒）。这种条件下路径能完整返回。4KB 接收缓冲是人为把窗口压满，不能当成客户程序的行为。

### 下次先看这些，再改配置

```powershell
ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@191.40.32.186 "caddy version; awk '/timeouts/,/}/' /etc/caddy/Caddyfile; systemctl is-active caddy"
ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@191.40.32.186 "journalctl -u caddy --since '2026-10-16 12:00:00' --no-pager | grep 'aborting with incomplete response'"
```

先分清 `reading: context canceled` 和 `writing: i/o timeout`，再决定要不要动 `write_idle`。健康检查用 `https://zerocode.kaynlab.asia/health`。本机如果走了代理，先清掉 `HTTP(S)_PROXY`，否则看到的不是香港。

## 丽萨 `lisa-hk`（2026-10-14 15:30 +08 起接客户流量）

客户入口 `https://zerocode.kaynlab.asia` 已改回将军澳 `191.40.32.186`（Dynadot 权威节点现答这个地址，TTL 300）。丽萨 `154.64.246.176` 上的 Caddy、证书和访问日志已删除，80/443 不再监听。不要把丽萨的地址发给客户。

| 项 | 值 |
|----|----|
| 主机名 | `lisa-hk` |
| 厂商 | 丽萨主机，香港三网直连进阶版，2 核 / 2 GB / 40 GB NVMe / 50 Mbps / 2000 GB，¥188/月 |
| 公网 IPv4 | `154.64.246.176` |
| SSH | `root@154.64.246.176` 端口 `25924`，密钥 `id_ed25519_lisa_hk`，密码登录已关 |
| 系统 | Ubuntu 24.04.1 LTS，内核 `6.8.0-48-generic`，BBR + fq |
| Caddy | 官方源 `2.11.7`，`/etc/caddy/Caddyfile`。站点 `zerocode.kaynlab.asia` 与 `http://154.64.246.176` |
| 防火墙 | UFW 只放行 `25924/tcp`、`80/tcp`、`443/tcp` |

```powershell
ssh -i $HOME\.ssh\id_ed25519_lisa_hk -p 25924 -o IdentitiesOnly=yes root@154.64.246.176
```

反代目标 `https://zerocode.kaynlab.com`，Host 仍是 `zerocode.kaynlab.com`，`flush_interval -1`，请求体 256 MB，读写超时 3600 秒。回源只用 HTTP/1.1。2026-10-14 18:33 +08 把 `versions 1.1 2` 改成 `versions 1.1`：HTTP/2 把所有对话复用到一条连接上，新的聊天请求要等几十秒才有响应头，用量日志里的首字仍然只有几秒。改完后从成都移动打 `kimi-k3-fast`，经香港首字 3.7 秒和 6.0 秒，直连美国 2.7 秒和 3.4 秒。Let's Encrypt 证书 CN=`zerocode.kaynlab.asia`，2026-10-14 签发，有效至 2027-01-12。切流后从成都移动直连（绕过本机代理）访问新地址：`/v1/models` 401，`/health` 200，`/api/v1/settings/public` 200，远端 IP `154.64.246.176`。

切流前同一下午的对比仍有效：丽萨热请求首字节大约 0.8–1.6 秒，将军澳大约 0.55–0.63 秒。用户要求先切过去看。阿里 `223.5.5.5` 和 `114.114.114.114` 在切流后一两分钟内仍可能缓存旧地址，那些请求继续由将军澳应答。

## 现状（2026-09-30 的交付记录）

下面几条是交付当天的状态。版本、协议和超时以「2026-10 优化全过程」为准，不要按这里把 Caddy 退回 2.11.4 或把 HTTP/2 打开。

- 已交付并完成第一期 Caddy。主机名 `hk-relay`，BBR + fq，UFW 只开 22/80/443，sshd 仅公钥（`PermitRootLogin prohibit-password`）。
- 当天是 Caddy 2.11.4，反代 `https://zerocode.kaynlab.com`，Host 为 `zerocode.kaynlab.com`，`flush_interval -1`，请求体 256 MB，读写超时 3600s。2026-10-16 已升级到 2.11.7。
- 域名 `kaynlab.asia` 于 2026-09-30 01:51 UTC 在 Dynadot 注册，NS 为 `ns1.dyna-ns.net` / `ns2.dyna-ns.net`。入口主机名 `zerocode.kaynlab.asia` A 记录 `191.40.32.186`。Let's Encrypt 证书已签发（CN=`zerocode.kaynlab.asia`，有效至 2026-12-29）。
- 线下发给客户：`https://zerocode.kaynlab.asia`。操作员 HTTP IP `http://191.40.32.186` 仍开着，不要发给客户。用户「API接入」不展示香港芯片（`custom_endpoints` 仍为 `[]`）。
- HTTPS 已验证：`/v1/models` → 401 `API_KEY_REQUIRED`；`/api/v1/settings/public` → 200。流式 `chat/completions` 还要用你自己的 key 打一条。
- 香港到源站 `curl -I https://zerocode.kaynlab.com` 为 HTTP/2 200，ping 约 228 ms。
- 2026-09-30 约 10:10 HKT，本机直连（清掉 `HTTP(S)_PROXY=127.0.0.1:10808`）测香港节点：ICMP 12/12 平均 132 ms（102–163）；TCP 80/443/22 各 5/5；HTTPS `/v1/models` 新建连接 15/15 均 401，中位合计约 1.0 s；keep-alive 后续约 310–330 ms。路由经中国移动 `221.183` / `223.120`（白天样本，不是晚高峰）。
- 晚高峰电信 / 联通 / 移动流式测试未做。未切用户。源站 `trusted_proxies` 未写入香港 IP。
- 本机对 `:22` 有时 `Test-NetConnection` 成功、有时 `ssh` 超时。超时不能单独当成机器宕机。

## 操作员自测

```powershell
curl.exe -sS -D - -o NUL -m 20 https://zerocode.kaynlab.asia/v1/models
curl.exe -sS -D - -o NUL -m 20 https://zerocode.kaynlab.asia/api/v1/settings/public
curl.exe -sS -D - -o NUL -m 20 http://191.40.32.186/v1/models
```

流式带你自己的 key：`https://zerocode.kaynlab.asia/v1/chat/completions`。

## API接入展示

2026-09-29 已把源站和本地 `custom_endpoints` 清回 `[]`。用户「API接入」只显示默认海外地址。

要重新展示香港入口：管理后台 → 系统设置 → 常规 → 自定义端点。名称填「香港」，地址填 `https://zerocode.kaynlab.asia`，保存整页。有香港行时，本地前端会把默认芯片标成「海外」（需部署后生产才改名）。

不要用 admin `PUT /api/v1/admin/settings` 的部分字段更新。后台页面是整份表单再保存，这条路径是安全的。

重装或改配置：

```powershell
scp -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes docs/dev/hk-relay/Caddyfile root@191.40.32.186:/etc/caddy/Caddyfile
ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@191.40.32.186 "caddy validate --config /etc/caddy/Caddyfile; systemctl reload caddy"
```

## 装好之后的基线

- 只开 22（密钥登录）、80、443。
- 开 BBR。
- Caddy：`flush_interval -1`，请求体至少 256 MB，读和写的总预算 3600 秒。对客户和回源都只用 HTTP/1.1。不要打开 `h2`，不要安装 2.11.6。`write_idle` 保持 60 秒，`write` 保持 3600 秒。
- 反代目标 `https://zerocode.kaynlab.com`，Host 固定为 `zerocode.kaynlab.com`。
- 源站可选把 `191.40.32.186` 写入 `server.trusted_proxies`，用量日志才能看到真实客户端 IP。

## 相关文档

- 机队：[`SERVERS.md`](SERVERS.md)
- 采购：[`HK_RELAY_BUYING_GUIDE.md`](HK_RELAY_BUYING_GUIDE.md)
- 源站事故：[`PRODUCTION.md`](PRODUCTION.md)
- 源站发版：[`DEPLOYMENT.md`](DEPLOYMENT.md)
- Trellis 任务：`.trellis/tasks/09-29-hk-caddy-relay/`
