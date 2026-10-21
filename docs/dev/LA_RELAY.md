# 洛杉矶中转 `la-relay`

核对日期：2026-10-21。客户域名 `zerocode.kaynlab.asia` 的 A 记录指向这台机器。香港 `191.40.32.186` 仍保留 TCP 拼接，只作回退，见 [`HK_RELAY.md`](HK_RELAY.md)。机队总表在 [`SERVERS.md`](SERVERS.md)。

## 名称

| 项 | 值 |
|----|----|
| 内部名称 | `la-relay` |
| 系统主机名 | `hk-relay-new` |
| 公网 IPv4 | `64.83.27.87` |
| 登录 | `root@64.83.27.87` |
| 本地 SSH 密钥 | `%USERPROFILE%\.ssh\id_ed25519_yt_hk` / `~/.ssh/id_ed25519_yt_hk` |
| 系统 | Debian 12 |

```powershell
ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@64.83.27.87
```

这台不跑 Sub2API、PostgreSQL、Redis，也不要 `docker compose` 或 `update.sh`。

## 客户入口

443 由 HAProxy 按 SNI 分流。仓库副本是 [`la-relay/haproxy.cfg`](la-relay/haproxy.cfg) 和 [`la-relay/Caddyfile`](la-relay/Caddyfile)。

| 客户怎么连 | 机器上怎么走 |
|------------|----------------|
| `https://zerocode.kaynlab.asia` | SNI 是域名。HAProxy 原样拼到 `15.204.102.44:8443`，PROXY v2。源站终结 TLS |
| `https://64.83.27.87` | 多数客户端连 IP 时不带 SNI。HAProxy 把「没有 SNI」、SNI 为这个 IP、以及 ACME 的 `acme-tls/1` 交给本机 Caddy `127.0.0.1:9443`。Caddy 用 IP 证书终结 TLS，再 HTTP 反代到源站 `8088` |
| `http://64.83.27.87:8088` | 本机 Caddy 直接收明文，再反代到源站 `8088` |

IP 证书的使用者可选名称是 `IP Address:64.83.27.87`，本次签发到 `2026-10-28 02:30 GMT`。Let's Encrypt 的 IP 证书走 `shortlived` 配置，大约 6.5 天，由 Caddy 自动续。厂商防火墙拦住了公网 80，所以续期用 `tls-alpn-01`，不要改回 `http-01`。

IP 的 HTTPS 只提供 HTTP/1.1。本机 Caddy 看到的对端是 HAProxy，所以这段用 PROXY v2，`{remote_host}` 才是客户地址。源站 `8088` 只在对端是 `64.83.27.87` 时采用这个 `X-Real-IP`。直接打源站的客户仍记 TCP 对端。

明文 HTTP 会把 API 密钥暴露在链路上。端口写 `8088`，不要写 `80`：这台的 80 从外网连不上，源站的 80 会 308 到 HTTPS。不要把 `8443` 给客户，那个口只收中转的 PROXY。

流式响应两边都是 `flush_interval -1`，不压缩 `text/event-stream`。
