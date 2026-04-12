# Sub2API 二次开发指南

## 一、开发环境搭建

### 1.1 前置依赖

| 工具 | 版本要求 | 安装方式 |
|------|----------|----------|
| Go | 1.26.2+ | https://go.dev/dl/ |
| Node.js | 20+ | https://nodejs.org/ |
| pnpm | latest | `npm install -g pnpm` |
| PostgreSQL | 16+ | 系统安装或 Docker |
| Redis | 7+ | 系统安装或 Docker |
| golangci-lint | v2.7+ | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7` |

### 1.2 Fork 与克隆

```bash
# 1. GitHub 上 Fork Wei-Shaw/sub2api 到自己账号
# 2. 克隆你的 Fork
git clone https://github.com/<your-username>/sub2api.git
cd sub2api

# 3. 添加上游远程
git remote add upstream https://github.com/Wei-Shaw/sub2api.git
```

### 1.3 启动依赖服务

**方式 A：Docker（推荐）**
```bash
docker-compose -f deploy/docker-compose.dev.yml up -d
```

**方式 B：本地安装**
- PostgreSQL: localhost:5432, user/pass=sub2api/sub2api, db=sub2api
- Redis: localhost:6379, 无密码

### 1.4 后端启动

```bash
cd backend

# 配置（首次需要）
cp ../deploy/config.example.yaml ./config.yaml
# 编辑 config.yaml，配置数据库和 Redis 连接

# 运行
go run ./cmd/server/
# 首次运行会进入 Setup Wizard，或设置 AUTO_SETUP=true 环境变量跳过
```

### 1.5 前端启动

```bash
cd frontend
pnpm install
pnpm dev
# 默认 http://localhost:3000，API 代理到后端 :8080
```

## 二、代码架构与二开指南

### 2.1 请求处理链路

```
HTTP Request
  → Gin Router (server/routes/)
    → Middleware (middleware/)
      → Handler (handler/)
        → Service (service/)
          → Repository (repository/)
            → Ent ORM → PostgreSQL
            → Redis Cache
```

### 2.2 常见二开场景

#### 场景 1：新增 API 端点

```
1. backend/internal/handler/      新建或修改 handler
2. backend/internal/service/      新建或修改 service 方法
3. backend/internal/repository/   如需新查询，添加 repo 方法
4. backend/internal/server/routes/ 注册路由
5. backend/cmd/server/wire.go     如有新依赖，更新 Wire 注入
6. go generate ./cmd/server       重新生成 wire_gen.go
```

#### 场景 2：修改数据库表结构

```
1. backend/ent/schema/            修改或新建 Schema
2. go generate ./ent              重新生成 Ent 代码
3. backend/migrations/            生成迁移文件
4. 提交生成的 ent/ 目录变更
```

#### 场景 3：新增前端页面

```
1. frontend/src/views/            新建 Vue 页面组件
2. frontend/src/router/           注册路由
3. frontend/src/api/              添加 API 调用
4. frontend/src/stores/           如需状态管理，添加 Pinia store
5. frontend/src/i18n/             添加国际化文本
```

#### 场景 4：新增支付方式

```
1. backend/internal/payment/provider/   实现 PaymentProvider 接口
2. backend/internal/payment/            注册到支付工厂
3. backend/internal/config/             添加配置字段
4. frontend/src/views/admin/            添加管理界面
```

### 2.3 Wire 依赖注入

项目使用 Google Wire 管理依赖。修改依赖关系后：

```bash
cd backend
go generate ./cmd/server   # 重新生成 wire_gen.go
```

关键文件：
- `backend/cmd/server/wire.go` — 依赖声明
- `backend/cmd/server/wire_gen.go` — 生成代码（不要手动编辑）

### 2.4 Ent ORM Schema

数据库模型定义在 `backend/ent/schema/`。修改后必须重新生成：

```bash
cd backend
go generate ./ent
git add ent/   # 生成的文件必须提交
```

## 三、同步上游更新

```bash
# 拉取上游最新代码
git fetch upstream

# 基于上游 main 同步
git checkout main
git merge upstream/main

# 解决冲突（如有）后推送
git push origin main

# 在功能分支上 rebase
git checkout feature/my-feature
git rebase main
```

### 合并策略建议

- **小改动**：直接在 main 上 merge upstream
- **大改动/长期分支**：定期 rebase upstream/main，减少最终合并冲突
- **关注的文件**：`go.mod`, `pnpm-lock.yaml`, `ent/` 目录是合并冲突高发区

## 四、测试规范

### 4.1 后端测试

```bash
# 单元测试（无外部依赖）
go test -tags=unit ./...

# 集成测试（需要 testcontainers / Docker）
go test -tags=integration ./...

# E2E 测试
go test -tags=e2e -v -timeout=300s ./internal/integration/...

# Lint
golangci-lint run ./...
```

### 4.2 前端测试

```bash
pnpm run test          # Vitest
pnpm run lint:check    # ESLint
pnpm run typecheck     # TypeScript
```

### 4.3 PR 提交前检查清单

- [ ] 后端单元测试通过：`go test -tags=unit ./...`
- [ ] 后端集成测试通过：`go test -tags=integration ./...`
- [ ] Lint 无新增问题：`golangci-lint run ./...`
- [ ] pnpm-lock.yaml 已同步（如改了 package.json）
- [ ] Ent 生成代码已提交（如改了 schema）
- [ ] Wire 生成代码已提交（如改了依赖注入）
- [ ] 所有 test stub 已补全新接口方法

## 五、构建与发布

### 5.1 本地构建 Docker 镜像

```bash
# 从项目根目录
docker build -t my-sub2api:latest .

# 测试运行
docker run -p 8080:8080 my-sub2api:latest
```

### 5.2 自建 CI/CD 发布流程

```
git tag v1.0.0-custom
git push origin v1.0.0-custom
# → GitHub Actions 触发 release.yml
# → GoReleaser 构建多架构二进制 + Docker 镜像
# → 推送到 DockerHub / GHCR
```

如果你 Fork 了项目，需要在 GitHub repo Settings → Secrets 中配置：
- `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` — DockerHub 推送
- 或使用 GHCR（无需额外配置，GitHub Token 自动可用）

### 5.3 手动发布到服务器

```bash
# 本地构建
make build

# 复制到服务器
scp backend/bin/server user@server:/opt/sub2api/sub2api

# 重启服务
ssh user@server "systemctl restart sub2api"
```
