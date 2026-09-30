# 香港中转 `hk-relay`

核对日期：2026-09-29。机器已开机，尚未接入用户流量。采购结论在 [`HK_RELAY_BUYING_GUIDE.md`](HK_RELAY_BUYING_GUIDE.md)。机队总表在 [`SERVERS.md`](SERVERS.md)。

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
| Caddy | 官方源 `2.11.4`，systemd `caddy`，配置 `/etc/caddy/Caddyfile`（仓库副本 [`hk-relay/Caddyfile`](hk-relay/Caddyfile)） |
| SSH 主机密钥 | `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/lfvm71fBLZs5oS7h8FCDHEENDd+HmGKP4ejsIv/jY`（2026-09-29 本机首次握手写入 `known_hosts`） |

```powershell
ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@191.40.32.186
```

## 用途

大陆客户经常打不开布法罗源站 `zerocode.kaynlab.com`（权威 DNS 在 Cloudflare）。这台机器只做反向代理：

```text
大陆客户端 → hk-relay:443（Caddy 终结 TLS）
          → https://zerocode.kaynlab.com （Host 仍为 zerocode.kaynlab.com）
          → buffalo-origin 172.245.247.80
```

上游模型流量留在 `buffalo-origin`。海外用户继续用 `zerocode.kaynlab.com`。

## 明确不做什么

- 不安装、不运行 Sub2API、PostgreSQL、Redis、AIClient2API、InvokeAI。
- 不跑现在这套 Docker Compose，不用 `update.sh`，不 pull GHCR。
- 不把 `kaynlab.com` 的 NS 改到这台机器上。
- 不在切流前把 `zerocode.kaynlab.com` 的 A 记录改到香港。
- 前面不套 Cloudflare。

## 现状（2026-09-30）

- 已交付并完成第一期 Caddy。主机名 `hk-relay`，BBR + fq，UFW 只开 22/80/443，sshd 仅公钥（`PermitRootLogin prohibit-password`）。
- Caddy 2.11.4 反代 `https://zerocode.kaynlab.com`，Host 仍为 `zerocode.kaynlab.com`，`flush_interval -1`，请求体 256 MB，读写超时 3600s。
- 域名 `kaynlab.asia` 于 2026-09-30 01:51 UTC 在 Dynadot 注册，NS 为 `ns1.dyna-ns.net` / `ns2.dyna-ns.net`。入口主机名定为 `zerocode.kaynlab.asia`。Caddy 已加上该站点块并监听 `:443`。Let's Encrypt 要等该名 A 记录指向 `191.40.32.186`（当前公开解析仍是停车页 `185.53.179.128`）。
- 第一期入口：`http://191.40.32.186`（操作员）。用户「API接入」不展示香港芯片；线下发给客户用 HTTPS 域名。
- 本机已验证：`/v1/models` → 401 `API_KEY_REQUIRED`；`/api/v1/settings/public` → 200；2 MB POST 未被 413；约 10.9 MB 静态资源下载约 14s。流式 `chat/completions` 还要用你自己的 key 打一条。
- 香港到源站 `curl -I https://zerocode.kaynlab.com` 为 HTTP/2 200，ping 约 228 ms。
- 晚高峰电信 / 联通 / 移动流式测试未做。未切用户。源站 `trusted_proxies` 未写入香港 IP。
- 本机对 `:22` 有时 `Test-NetConnection` 成功、有时 `ssh` 超时。超时不能单独当成机器宕机。

## 操作员自测

```powershell
curl.exe -sS -D - -o NUL -m 20 http://191.40.32.186/v1/models
curl.exe -sS -D - -o NUL -m 20 http://191.40.32.186/api/v1/settings/public
```

流式需要带你自己的 key，走 `http://191.40.32.186/v1/chat/completions`。Authorization 是明文。

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
- Caddy：第一期听 `http://191.40.32.186:80`。`flush_interval -1`，请求体至少 256 MB，读超时 3600 秒或关闭，HTTP/2 和 WebSocket。
- 反代目标 `https://zerocode.kaynlab.com`，Host 不改写。
- 源站可选把 `191.40.32.186` 写入 `server.trusted_proxies`，用量日志才能看到真实客户端 IP。

## 相关文档

- 机队：[`SERVERS.md`](SERVERS.md)
- 采购：[`HK_RELAY_BUYING_GUIDE.md`](HK_RELAY_BUYING_GUIDE.md)
- 源站事故：[`PRODUCTION.md`](PRODUCTION.md)
- 源站发版：[`DEPLOYMENT.md`](DEPLOYMENT.md)
- Trellis 任务：`.trellis/tasks/09-29-hk-caddy-relay/`
