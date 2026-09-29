# Design: hk-relay Caddy 中转

## Architecture

两台机器，一条转发链：

```text
第一期（操作员测试）
  客户端 → http://191.40.32.186:80
        → hk-relay Caddy
        → https://zerocode.kaynlab.com  (Host: zerocode.kaynlab.com)
        → buffalo-origin 172.245.247.80

第二期（给大陆用户）
  客户端 → https://api.<新域名>
        → 同上
```

海外客户端继续直连 `zerocode.kaynlab.com`。上游 AI 请求只从源站出去。

## Boundaries

| 机器 | 跑什么 | 不跑什么 |
|------|--------|----------|
| `hk-relay` | Caddy、ssh、防火墙 | Sub2API、Postgres、Redis、Docker、`update.sh` |
| `buffalo-origin` | 现网全部 | 不承担大陆 DNS 入口 |

仓库代码默认不改网关协议。可能改动：源站 `server.trusted_proxies`、文档、以及香港机上的 Caddyfile（不进 Sub2API 容器）。

## Caddy 合同

与采购说明一致，香港与源站同一组限制：

- `reverse_proxy https://zerocode.kaynlab.com`
- 上游 TLS 校验保留；`header_up Host zerocode.kaynlab.com`
- `flush_interval -1`
- `request_body` 至少 256 MB
- 读超时 3600s 或关闭
- 打开 HTTP/2 与 WebSocket

Caddyfile 放在香港 `/etc/caddy/Caddyfile`，由 Caddy 官方 Debian 源安装。

第一期站点块用 `http://191.40.32.186`，不开自动 HTTPS。Let's Encrypt 不签裸 IP。自签证书只适合 `curl -k`，OpenAI SDK / Claude Code / 酒馆默认会验证书失败。香港到源站这一跳仍是 HTTPS。

第二期加上 `api.<新域名>` 站点块，Caddy 自动 HTTPS。IP 的 HTTP 入口可以关掉，避免用户误用明文。

## 系统基线

- `hostnamectl set-hostname hk-relay`
- `net.ipv4.tcp_congestion_control=bbr`，`net.core.default_qdisc=fq`
- `sshd`: 公钥、禁止密码、禁止 root 密码；本任务继续用 root 密钥（厂商默认）
- nftables/ufw：22、80、443；其余丢弃
- 不装 Docker

## DNS 与证书

第一期没有 DNS。客户端直连 `191.40.32.186`，正好绕过 Cloudflare 权威解析问题，适合测线路。

第二期：

1. 注册独立 `.com` / `.net`。
2. NS 留在 DNSPod 或阿里云。
3. `api.<域>` A → `191.40.32.186`，TTL 120。
4. Caddy 申请证书。
5. 晚高峰三网测通后再把 HTTPS URL 发给大陆用户。

`kaynlab.com` 的权威 DNS 不动。不要用 `*.sslip.io` 当正式入口（源站 InvokeAI 调试用过，大陆解析和证书都不当生产）。

## 源站

可选：`deploy` 侧 `server.trusted_proxies` 加上 `191.40.32.186`，否则用量日志里客户端 IP 全是香港。此项单独一次生产配置变更，需要当次允许。

## Rollback

- 第一期：停 Caddy 或关掉 80 即可，没有用户入口。
- 第二期切流前：关掉香港 Caddy 或删 A 记录，无用户影响。
- 切流后：大陆用户改回 `zerocode.kaynlab.com`（他们本来就打不开的会回到原状）；源站 compose 不回滚。
- 香港机器可以留着继续测线路。

## Risks

- 云途 TKO 是尽力而为优化，晚高峰某一网可能丢包。失败则按采购顺序看 VMISS / DMIT，不在本机加核。
- 本机 SSH 到香港不稳定。实施时先确认 22 通，再改防火墙，避免把自己锁在门外（先留控制台密码到密钥登录验证完成）。
- 1000 GB 配额按厂商口径；超额规则以控制台为准。
