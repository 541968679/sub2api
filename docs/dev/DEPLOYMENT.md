# Sub2API 部署运维手册

## 一、部署架构概览

```
用户请求 → Caddy/Nginx (反代+SSL) → Sub2API (:8080) → 上游 AI API
                                        ↓
                                   PostgreSQL + Redis
```

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
