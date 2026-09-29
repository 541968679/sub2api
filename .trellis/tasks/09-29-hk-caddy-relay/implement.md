# Implement: hk-relay

未 `task.py start`。第一期用 `http://191.40.32.186`，域名不阻塞装 Caddy。用户切流仍等 HTTPS 域名。

## Checklist

1. **机队记录**（完成）  
   `docs/dev/SERVERS.md`、`docs/dev/HK_RELAY.md`，以及 `AGENTS.md` / `PRODUCTION.md` / `DEPLOYMENT.md` 指针。

2. **登录与基线**（完成 2026-09-29）  
   - `ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@191.40.32.186`  
   - 主机名 `hk-relay`（`preserve_hostname: true`）  
   - BBR + fq  
   - 密钥登录后再关密码登录  
   - UFW 22/80/443  
   - 验证：`hostname` = `hk-relay`，`tcp_congestion_control` = `bbr`，`ss` 有 `:22` 和 `:80`

3. **装 Caddy（第一期，完成 2026-09-29）**  
   官方 Cloudsmith 源 Caddy 2.11.4。香港 `curl -I https://zerocode.kaynlab.com` 为 HTTP/2 200。  
   Caddyfile 站点 `http://191.40.32.186`，`flush_interval -1`，读超时 3600s，Host 仍为 `zerocode.kaynlab.com`。不开自动 HTTPS。仓库副本 `docs/dev/hk-relay/Caddyfile`。

4. **功能测试（IP）**  
   - `http://191.40.32.186/v1/models` → 401 `API_KEY_REQUIRED`（完成）  
   - 约 2 MB 上传未被 413；约 10.9 MB 下载约 14s（完成）  
   - 一条流式 `chat/completions`（未做，需要操作员自己的 key，明文 HTTP）

4b. **API接入芯片**（完成 2026-09-29）  
   源站和本地 `custom_endpoints` 写入 `香港` → `http://191.40.32.186`。用户页芯片类型为「香港」。有香港行时默认芯片标「海外」（需前端部署后生产才改名）。

5. **域名（第二期，用户切流前）**  
   注册 `.com`/`.net`，NS 在 DNSPod/阿里云，`api.<域>` A `191.40.32.186` TTL 120。加上 HTTPS 站点块，再关掉或限制 IP 的明文 80。

6. **功能测试（域名）**  
   `https://api.<域>/v1/models` 证书匹配；再打一条流式。

7. **晚高峰**  
   20:00–23:00 CST，电信 / 联通 / 移动。一网明显失败：停切流，换线路方案，不在本机加核。

8. **源站 trusted_proxies**（可选，需当次允许）  
   写入 `191.40.32.186`，核对用量日志 IP。

9. **切流**（需当次允许）  
   只把大陆入口改成新 URL。不改 `zerocode.kaynlab.com` A 记录。

## Validation

```powershell
ssh -i $HOME\.ssh\id_ed25519_yt_hk -o IdentitiesOnly=yes root@191.40.32.186 "hostname; sysctl net.ipv4.tcp_congestion_control; ss -lnt"
```

第一期：对 `http://191.40.32.186/v1/models` 期望与源站同类的鉴权或成功响应。第二期再验 `https://api.<新域名>` 证书。

## Rollback

步骤 4 之前：停 Caddy 即可。步骤 9 之后：大陆用户改回旧 URL；源站 DNS 与 compose 不动。

## Risky points

- 改防火墙前确认控制台 VNC/密码还能进。
- 不要在 `hk-relay` 上跑 `update.sh`。
- 不要把源站密钥拷到香港。
