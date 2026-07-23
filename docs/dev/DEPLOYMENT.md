# Sub2API 部署运维手册

## 一、部署架构概览

```
用户请求 → Caddy/Nginx (反代+SSL) → Sub2API (:8080) → 上游 AI API
                                        ↓
                                   PostgreSQL + Redis
```

### 1.1 本项目生产环境速查

生产服务器与常用部署入口记录在这里，避免只留在聊天记录中。更完整的 Kiro/AIClient2API 侧车说明见 `docs/dev/KIRO_PROXY.md`；InvokeAI 侧车说明见 `docs/dev/INVOKEAI_SIDECAR.md`。

| 项目 | 值 |
|------|----|
| 生产服务器 | `root@172.245.247.80` |
| 本地 SSH key | `%USERPROFILE%\.ssh\id_ed25519_sub2api` / `~/.ssh/id_ed25519_sub2api` |
| Compose 目录 | `/opt/sub2api` |
| Sub2API 主服务镜像 | `ghcr.io/541968679/sub2api:latest` |
| Sub2API 镜像覆盖变量 | `SUB2API_IMAGE` |
| Sub2API 源码目录 | `/opt/sub2api/repo`（历史本机构建目录；后续主服务部署不得依赖它构建镜像） |
| AIClient2API 镜像 | `ghcr.io/541968679/aiclient2api:latest` |
| AIClient2API 镜像覆盖变量 | `AICLIENT2API_IMAGE` |
| AIClient2API 配置目录 | `/opt/aiclient2api/configs` |
| InvokeAI 镜像 | `ghcr.io/541968679/invokeai-sub2api:latest` |
| InvokeAI 镜像覆盖变量 | `INVOKEAI_IMAGE` |
| InvokeAI 数据目录 | `/opt/invokeai/root` |
| InvokeAI 公网调试入口 | `https://invokeai.172.245.247.80.sslip.io` |
| 部署日志 | `/opt/sub2api/deploy.log` |

常用命令：

Sub2API 主服务后续必须使用 GitHub Actions 发布到 GHCR 的镜像。不要在生产服务器执行 `docker build`，不要部署 `sub2api-custom:*`。如果服务器上的 `/opt/sub2api/update.sh` 仍会为主服务本机构建镜像，则该脚本在改造前只能用于侧车部署，不能用于 Sub2API 主服务部署。

```powershell
# 只部署 Sub2API 主服务（GHCR；执行前确认 docker compose config 中 sub2api.image 指向 GHCR）
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose pull sub2api && docker compose up -d --no-deps sub2api"

# 只部署 AIClient2API 侧车
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh --only-a2"

# 只部署 InvokeAI 侧车
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh --only-invokeai"

# 完整部署 Sub2API + AIClient2API + InvokeAI（GHCR；执行前确认三者镜像均可 pull）
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose pull sub2api aiclient2api invokeai && docker compose up -d sub2api aiclient2api invokeai"
```

部署后核对：

```powershell
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose ps"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "docker inspect sub2api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}}'"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose logs --tail=120 aiclient2api"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose logs --tail=120 invokeai"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "tail -n 120 /opt/sub2api/deploy.log"
```

注意事项：

- Sub2API 主服务镜像由 GitHub Actions 构建并发布到 GHCR；生产部署只允许 `docker compose pull/up` 已发布镜像，不允许在生产服务器 `docker build`。
- 当前 Release workflow 只会在 `v*` tag 推送或手动 `workflow_dispatch` 时发布 GHCR 镜像；单独 push `main` 不会刷新 `ghcr.io/541968679/sub2api:latest`。生产 `pull/up` 前必须确认目标 tag 或 `latest` 已经存在，并且镜像 label 指向本次要部署的 commit。
- 下次主服务部署前，必须清理或替换历史 `/opt/sub2api/docker-compose.override.yml` 中对 `sub2api-custom:latest` 的 pin，确保 `docker compose config` 解析出的 `sub2api.image` 是 `ghcr.io/541968679/sub2api:latest` 或本次明确批准的 GHCR tag。
- 生产 AIClient2API 是 sub2api Compose 中的侧车服务，服务名为 `aiclient2api`，宿主机仅绑定 `127.0.0.1:3000`。
- Sub2API 内部访问 AIClient2API 使用 `http://aiclient2api:3000/claude-kiro-oauth`，不要改成本机公网地址。
- AIClient2API 镜像由 GitHub Actions 构建并发布到 GHCR；`deploy/update.sh --only-a2` 只执行 `docker compose pull aiclient2api` 和重启。
- 生产 InvokeAI 是 sub2api Compose 中的侧车服务，服务名为 `invokeai`，宿主机仅绑定 `127.0.0.1:9090`；公网访问必须通过 Caddy/Nginx 反代。
- InvokeAI 镜像由 GitHub Actions 以 `GPU_DRIVER=cpu` 构建并发布到 GHCR；`deploy/update.sh --only-invokeai` 只执行 `docker compose pull invokeai` 和重启。
- InvokeAI 只作为外部 API 生图客户端使用，生产配置必须保持 `INVOKEAI_DEVICE=cpu` 和 `INVOKEAI_PRECISION=float32`，不要引入本地模型/GPU 推理。
- 如果 GHCR package 没有设为 Public，生产服务器需要先 `docker login ghcr.io`。
- 不要把生产 API key、Web UI 密码、代理订阅等敏感信息写入本文档或提交到 Git。

### 1.2 Sub2API 主服务 release / deploy 流程

本仓库当前的 `.github/workflows/release.yml` 只监听 `v*` tag 和手动
`workflow_dispatch`。`.goreleaser.simple.yaml` 会发布
`ghcr.io/541968679/sub2api:<version>`、`:<version>-amd64` 和 `:latest`。

主服务生产部署按这个顺序执行：

1. 将已经验证的代码合入并推送到 `main`。
2. 创建并推送下一个 `v*` tag，或手动触发 Release workflow。
3. 等 GitHub Actions Release 成功后，确认 GHCR 镜像已经发布。
4. 确认生产 Compose 的 `sub2api.image` 指向 GHCR，而不是
   `sub2api-custom:*`。
5. 在生产机执行 `docker compose pull sub2api` 和
   `docker compose up -d --no-deps sub2api`。
6. 核对运行镜像、revision/version label、容器健康状态和 `/health`。

常用检查命令：

```powershell
# 确认目标 tag 可拉取
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "docker manifest inspect ghcr.io/541968679/sub2api:0.1.137 >/dev/null && echo manifest-ok"

# 确认生产 compose 最终解析到 GHCR 镜像
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose config | grep -A 5 'sub2api:'"
```

PowerShell 下跨 SSH 检查 `docker inspect --format` 时，优先用 heredoc，避免
本地引号转义破坏 Go template：

```powershell
$script = @'
set -e
docker inspect sub2api --format 'image={{.Config.Image}}'
docker inspect sub2api --format 'image_id={{.Image}}'
docker inspect sub2api --format 'status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}no-health{{end}}'
docker inspect sub2api --format 'revision={{index .Config.Labels "org.opencontainers.image.revision"}}'
docker inspect sub2api --format 'version={{index .Config.Labels "org.opencontainers.image.version"}}'
wget -q -T 5 -O - http://127.0.0.1:8080/health
'@
$script | ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 'bash -s'
```

最近一次已验证主服务生产部署：

| 日期 | Tag | Revision | Image | Version label | 状态 |
|------|-----|----------|-------|---------------|------|
| 2026-07-23 | `v0.1.171` | `151d3b4caee611b6ad4bb6c7c4bcb4705939fda6` | `ghcr.io/541968679/sub2api:latest` | `0.1.171` | running, healthy, `/health` OK |
| 2026-07-23 | `v0.1.170` | `db26feddbcd87ebc3722fb7f0d740d38c4f10e5e` | `ghcr.io/541968679/sub2api:latest` | `0.1.170` | running, healthy, `/health` OK |
| 2026-07-17 | `v0.1.169` | `e9f6938331283c2c0d5ea07f82bc46bb9025f0c7` | `ghcr.io/541968679/sub2api:latest` | `0.1.169` | running, healthy, `/health` OK |
| 2026-07-15 | `v0.1.168` | `f38c7f0d5ffb8d4f4af21317a144de45f220ba28` | `ghcr.io/541968679/sub2api:latest` | `0.1.168` | running, healthy, `/health` OK |
| 2026-07-15 | `v0.1.165` | `cddca2a7cf70e43d8b5bc0c4fa68aa43ad4cfbc8` | `ghcr.io/541968679/sub2api:latest` | `0.1.165` | running, healthy, `/health` OK |
| 2026-06-03 | `v0.1.137` | `e385b9ac7d7e840658cbcb4f7f9f8f11b1954b81` | `ghcr.io/541968679/sub2api:latest` | `0.1.137` | running, healthy, `/health` OK |

## 二、Docker Compose 部署（推荐）

### 2.1 环境要求

- Linux (Ubuntu 22.04+ / Debian 12+ 推荐)
- Docker 24+ & Docker Compose v2
- 最低 2C4G，推荐 4C8G（商用）

### 2.2 一键部署

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh | bash
```

脚本自动完成：
1. 下载 `docker-compose.local.yml` 和 `.env.example`
2. 生成安全密钥（JWT_SECRET, TOTP_ENCRYPTION_KEY, POSTGRES_PASSWORD）
3. 创建数据目录
4. 等待手动 `docker-compose up -d`

### 2.3 手动部署

```bash
# 1. 准备目录
mkdir -p /opt/sub2api && cd /opt/sub2api

# 2. 获取配置
wget https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-compose.yml
wget https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/.env.example -O .env

# 3. 编辑 .env（必须修改的项）
vim .env
# POSTGRES_PASSWORD=<强密码>
# JWT_SECRET=<openssl rand -hex 32>
# TOTP_ENCRYPTION_KEY=<openssl rand -hex 32>
# ADMIN_EMAIL=admin@yourdomain.com
# ADMIN_PASSWORD=<强密码>

# 4. 启动
docker-compose up -d

# 5. 查看日志
docker-compose logs -f sub2api
```

### 2.4 更新版本

```bash
cd /opt/sub2api
docker-compose pull
docker-compose up -d
```

### 2.5 数据备份

```bash
# 备份 PostgreSQL
docker exec sub2api-postgres pg_dump -U sub2api sub2api > backup_$(date +%Y%m%d).sql

# 备份 Redis
docker exec sub2api-redis redis-cli BGSAVE

# 备份全部卷
docker run --rm -v sub2api_postgres_data:/data -v $(pwd):/backup alpine tar czf /backup/pg_data.tar.gz /data
```

### 2.6 数据恢复

```bash
# 恢复 PostgreSQL
cat backup_20260412.sql | docker exec -i sub2api-postgres psql -U sub2api sub2api
```

## 三、二进制 + systemd 部署

### 3.1 安装

```bash
# 自动检测架构、下载、安装、创建 systemd 服务
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | bash

# 安装路径
# 二进制: /opt/sub2api/sub2api
# 配置:   /etc/sub2api/config.yaml
# 服务:   sub2api.service
```

### 3.2 管理

```bash
systemctl start sub2api
systemctl stop sub2api
systemctl restart sub2api
systemctl status sub2api
journalctl -u sub2api -f   # 查看日志
```

### 3.3 升级/回滚

```bash
# install.sh 支持
bash install.sh upgrade
bash install.sh rollback
```

## 四、反向代理配置

### Caddy（推荐，自动 HTTPS）

```caddyfile
api.yourdomain.com {
    reverse_proxy localhost:8080
}
```

### Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # 大请求体（AI API 场景）
        client_max_body_size 256m;
        proxy_read_timeout 300s;
    }
}
```

## 五、生产环境优化

### 5.1 PostgreSQL 调优

```yaml
# config.yaml 或环境变量
DATABASE_MAX_OPEN_CONNS: 256      # 高并发场景
DATABASE_MAX_IDLE_CONNS: 64
DATABASE_CONN_MAX_LIFETIME_MINUTES: 30
```

系统级（postgresql.conf）：
```
shared_buffers = 2GB              # 25% of RAM
effective_cache_size = 6GB        # 75% of RAM
work_mem = 64MB
maintenance_work_mem = 512MB
max_connections = 500
```

### 5.2 Redis 调优

```yaml
REDIS_POOL_SIZE: 1024
REDIS_MIN_IDLE_CONNS: 128
```

### 5.3 系统级

```bash
# 文件描述符限制
echo "* soft nofile 100000" >> /etc/security/limits.conf
echo "* hard nofile 100000" >> /etc/security/limits.conf

# TCP 调优
sysctl -w net.core.somaxconn=65535
sysctl -w net.ipv4.tcp_max_syn_backlog=65535
```

## 六、监控与告警

### 6.1 健康检查

```bash
# HTTP 健康端点
curl http://localhost:8080/health
```

### 6.2 日志管理

```yaml
# config.yaml
log:
  level: info           # debug/info/warn/error
  format: json          # json/console
  output: stdout        # stdout 或文件路径
```

### 6.3 推荐监控方案

- 容器监控：Portainer / ctop
- 系统监控：Prometheus + Grafana / Node Exporter
- 日志聚合：Loki / ELK
- 告警：Grafana Alerting / Uptime Kuma（HTTP 健康检查）

## 七、安全加固清单

- [ ] 修改默认管理员密码
- [ ] 设置固定 JWT_SECRET 和 TOTP_ENCRYPTION_KEY
- [ ] 启用 HTTPS（Caddy 自动 / Nginx + Let's Encrypt）
- [ ] 启用 URL 白名单 (`SECURITY_URL_ALLOWLIST_ENABLED=true`)
- [ ] 关闭 debug 模式 (`SERVER_MODE=release`)
- [ ] PostgreSQL 不暴露外部端口（仅 Docker 内网）
- [ ] Redis 设置密码 (`REDIS_PASSWORD`)
- [ ] 定期运行 `make secret-scan`
- [ ] 配置防火墙，仅开放 80/443
