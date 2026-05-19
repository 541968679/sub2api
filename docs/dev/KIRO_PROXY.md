# Kiro 反向代理对接方案

## 背景

Sub2API 原生支持 Anthropic / OpenAI / Gemini / Antigravity 四个平台，但不支持 Kiro（AWS Q Developer）。
通过引入 [AIClient2API](https://github.com/justlovemaki/AIClient2API) 作为中间层，将 Kiro 账号反代为 Anthropic Messages API 兼容端点，再以 API Key 方式接入 Sub2API。

## 架构

```
用户请求 → Sub2API (Anthropic API Key 账号)
              ↓ POST {base_url}/v1/messages
         AIClient2API (/claude-kiro-oauth/v1/messages)
              ↓ Kiro OAuth + 格式转换
         AWS Kiro API (CodeWhisperer)
              ↓
         Claude Sonnet/Opus 系列
```

## 子项目位置

- 路径: `E:\cursor project\AIClient2API`
- 仓库: https://github.com/justlovemaki/AIClient2API
- 管理界面: http://localhost:3000 (默认密码: `admin123`)

## Sub2API 对接配置（已跑通）

在 Sub2API 管理后台添加账号:

| 字段 | 值 |
|---|---|
| 平台 | Anthropic |
| 账号类型 | API Key (Claude Console) |
| API Key | `123456`（AIClient2API 的 `REQUIRED_API_KEY`） |
| Base URL | `http://127.0.0.1:3000/claude-kiro-oauth` |
| 自动透传 | 开启 (`extra.anthropic_passthrough: true`) |

## AIClient2API 配置要点

### 本地启动

```bash
cd "E:\cursor project\AIClient2API"
npm install
npm start
# → http://localhost:3000
```

### 配置文件

- `configs/config.json` — 主配置，`PROXY_URL: "http://127.0.0.1:10809"` 让 Kiro 请求走代理
- `configs/provider_pools.json` — Kiro 账号池
- `configs/kiro/{ts}_kiro-auth-token/{ts}_kiro-auth-token.json` — OAuth 凭据

### Kiro 账号获取

1. 打开 Kiro IDE → 登录 Google 账号（注意：**登录前必须关闭 HTTP_PROXY/HTTPS_PROXY 环境变量**，否则 OAuth token 交换会失败）
2. 登录成功后 token 文件生成于 `~/.aws/sso/cache/kiro-auth-token.json`
3. 拷贝到 AIClient2API 的 `configs/kiro/` 目录（带时间戳子目录）
4. 在 `configs/provider_pools.json` 中添加 `claude-kiro-oauth` 节点指向该文件

### 已知坑

- **OAuth 登录代理冲突**：Kiro IDE 登录时回调到 localhost，但 token 交换走外网。系统代理/HTTP_PROXY 变量会破坏这一步。解决方案是用代理软件的 **TUN 模式**（流量在网络层透明转发，不依赖 HTTP_PROXY 变量）。
- **中国 IP 模型受限**：Kiro Pro 账号在中国 IP 登录时模型列表被限制为国产模型（DeepSeek/Qwen 等），看不到 Claude。必须通过美国节点代理（TUN 模式）登录。
- **TUN 模式下 AIClient2API 的 HTTPS 请求可能被劫持**：表现为 `The plain HTTP request was sent to HTTPS port` 400 错误。解决方案是在 AIClient2API 的 `config.json` 设置 `PROXY_URL` 让 Kiro 请求走独立代理端口，然后关掉 TUN。

## Sub2API 侧的二开改动

### 已完成（anthropic 分组，已跑通）

**`backend/internal/service/gateway_service.go`**：
1. passthrough 转发前清理模型名中的 `[1m]`/`[2m]` 等上下文窗口后缀。Claude Code 客户端会带这种后缀（如 `claude-opus-4-7[1m]`），Kiro/AIClient2API 不识别会报 400。
2. antigravity 分组选不到账号时，回退到 anthropic passthrough 账号（为 antigravity 分组接入 Kiro 而加，见下方未完成部分）。

**前端**：
- `CreateAccountModal.vue` / `EditAccountModal.vue`: 扩展 `anthropic_passthrough` 开关到 antigravity 平台 apikey 账号（本方案 A 下已无必要，但改动保留不影响正常使用）。

## 已完成：antigravity 分组接入 Kiro（方案 B）

### 需求

客户已有大量 antigravity 分组的 API Key 在用，希望不改 URL（继续用 `/antigravity/v1/messages`）就能切换到 Kiro 上游。

### 最终方案：Kiro 账号直接作为 antigravity 平台 passthrough 账号

放弃方案 A（路由层回退），改用方案 B：将 Kiro 账号配置为 `platform=antigravity` + `type=apikey` + `passthrough=true`，直接参与 antigravity 分组调度。

### 代码改动

1. **`account.go` — `IsAnthropicAPIKeyPassthroughEnabled()`**：放宽平台限制，从只接受 `anthropic` 改为同时接受 `antigravity`。
2. **`account.go` — `GetBaseURL()`**：antigravity passthrough 账号不再自动拼接 `/antigravity` 后缀（该后缀仅用于真正的 Google Cloud Code API Key 账号）。
3. **`gateway_service.go` — `isModelSupportedByAccountWithContext()` / `isModelSupportedByAccount()`**：antigravity passthrough 账号跳过模型映射检查，接受所有模型（与 OpenAI passthrough 同理）。
4. **`antigravity/claude_types.go` — `DefaultModels()`**：为 Claude 模型生成 `[1m]`/`[2m]` 上下文窗口后缀变体，供 Claude Code 客户端模型校验通过。
5. **`frontend/vite.config.ts`**：Vite 代理新增 `/antigravity` 路径转发。

### 请求链路

```
Claude Code 客户端 → /ity/v1/messages (ForcePlatform="antigravity")
Claude Code 客户端 → /antigravity/v1/messages (ForcePlatform="antigravity")
  → SelectAccountWithLoadAwareness (load-aware 路径)
  → listSchedulableAccounts(platform="antigravity") → 找到 Kiro passthrough 账号
  → isModelSupportedByAccountWithContext → passthrough bypass → true
  → handler: platform=antigravity + type=apikey → gatewayService.Forward
  → IsAnthropicAPIKeyPassthroughEnabled → true
  → 清理 [1m] 后缀 → 应用账号级模型映射 → 转发到 AIClient2API base_url
```

### 管理后台配置

| 字段 | 值 |
|---|---|
| 平台 | Antigravity |
| 账号类型 | API Key |
| API Key | AIClient2API 的 `REQUIRED_API_KEY` |
| Base URL | `http://aiclient2api:3000/claude-kiro-oauth`（生产）/ `http://127.0.0.1:3000/claude-kiro-oauth`（本地） |
| 自动透传 | 开启 |
| 模型映射 | 可选，如 `claude-opus-4-7 → claude-opus-4-6` |

### 排查过程中发现的坑

1. **load-aware 路径无回退**：之前的方案 A 回退逻辑写在 `SelectAccountForModelWithExclusions`（simple 路径），但生产环境走 load-aware 路径（`concurrencyService != nil && LoadBatchEnabled`），回退代码永远不会执行。
2. **`GetBaseURL()` 自动拼 `/antigravity`**：antigravity 平台的 apikey 账号会在 base_url 后自动拼接 `/antigravity`，导致 passthrough 请求 404。修复：passthrough 账号跳过此逻辑。
3. **`/antigravity/v1/models` 缺少 `[1m]` 变体**：Claude Code 客户端用带 `[1m]` 后缀的模型名校验 models 列表，匹配不到就拒绝使用。修复：生成后缀变体。
4. **Vite 代理未转发 `/antigravity`**：本地开发时前端 dev server 不代理 `/antigravity/` 路径，请求直接 404。

### 遗留（方案 A 代码保留）

`gateway_service.go` 第 1411 行附近的 antigravity→anthropic 回退逻辑保留不删，不影响正常运行。方案 B 下该代码不会被触发（antigravity 分组已有可用账号）。

## 提示词注入（已处理）

AIClient2API 的 `src/providers/claude/claude-kiro.js:1049` 在系统提示词前强制注入了一段 `<CRITICAL_OVERRIDE>`，原版会让模型自称"开发者何夕2077"。已改为让模型自称请求中传入的 `${model}` 名（如 `claude-opus-4-7`），与用户在 Claude Code 中选择的模型一致。

保留 `CRITICAL_OVERRIDE` 其他内容（不承认 Kiro 身份），避免用户发现模型来自 Kiro。

## 文件清单

| 文件 | 改动 |
|---|---|
| `backend/internal/service/gateway_service.go` | 模型名后缀清理、antigravity 回退逻辑 |
| `frontend/src/components/account/CreateAccountModal.vue` | 扩展 passthrough 开关到 antigravity |
| `frontend/src/components/account/EditAccountModal.vue` | 同上 |
| `deploy/docker-compose.yml` | 新增 aiclient2api service |
| `deploy/update.sh` | 扩展为同时部署 aiclient2api sidecar，支持 --skip-a2 / --only-a2 |
| `E:\cursor project\AIClient2API\src\providers\claude\claude-kiro.js` | 修改身份注入为动态 `${model}` |

## 生产环境部署

### 仓库 Fork

| 上游 | Fork | 角色 |
|---|---|---|
| `Wei-Shaw/sub2api` | `541968679/sub2api` | sub2api 主项目 |
| `justlovemaki/AIClient2API` | `541968679/AIClient2API` | Kiro 反代 sidecar |

### 构建方式

AIClient2API 与 sub2api 对齐为 CI 构建镜像、生产服务器拉取镜像部署：

```
git push main → GitHub Actions buildx → ghcr.io/541968679/aiclient2api:latest → docker compose pull → docker compose up -d
```

生产默认镜像：`ghcr.io/541968679/aiclient2api:latest`。如需临时切换镜像，可在生产 `.env` 设置 `AICLIENT2API_IMAGE`。

如果 GHCR package 没有设为 Public，生产服务器需要先执行一次 `docker login ghcr.io`，否则 `docker compose pull aiclient2api` 会因为无权限失败。

### 生产目录结构

```
/opt/sub2api/
├── repo/                          # sub2api 源码 (git clone)
├── docker-compose.yml             # 主 compose（包含两个 service）
├── update.sh                      # 部署脚本，A2 通过 docker compose pull 更新
├── .env                           # 环境变量（不进 git）
└── deploy.log

/opt/aiclient2api/
├── configs/
│   ├── config.json                # 主配置（含 REQUIRED_API_KEY 强口令）
│   ├── pwd                        # Web UI 登录密码
│   ├── provider_pools.json        # Kiro 账号池
│   └── kiro/initial/
│       └── kiro-auth-token.json   # Kiro OAuth 凭据
└── logs/
```

### 网络架构

```
Internet
   ↓ HTTPS
Caddy :443 (host systemd service, auto Let's Encrypt)
   ├─ zerocode.kaynlab.com   → 127.0.0.1:8080  (sub2api API)
   └─ a2.zerocode.kaynlab.com → 127.0.0.1:3000  (AIClient2API Web UI)
   ↓
Docker bridge (sub2api-network)
   ├─ sub2api:8080      (host-exposed :8080)
   └─ aiclient2api:3000 (host-exposed 127.0.0.1:3000 only, 公网不可达)
         ↓ 内部调用：http://aiclient2api:3000/claude-kiro-oauth/v1/messages
sub2api gateway → aiclient2api
         ↓
         ↓ https://q.us-east-1.amazonaws.com (server IP)
         Kiro API
```

#### 可选：AIClient2API 出站代理 sidecar

当同一台生产服务器 IP 触发上游 429 时，可以在 Docker 网络内增加 `a2-proxy`
sidecar，让 AIClient2API 通过独立代理出口访问上游：

```
aiclient2api
   ↓ PROXY_URL=http://a2-proxy:10809
a2-proxy (sing-box, Docker internal only)
   ↓ configured outbound node
Upstream AI API
```

仓库提供两个文件：

| 文件 | 作用 |
|---|---|
| `deploy/docker-compose.a2-proxy.yml` | 可选 compose override，新增 `a2-proxy` 服务 |
| `deploy/a2-proxy/sing-box.config.json.example` | sing-box 占位配置，默认 direct-only |

生产启用步骤：

```bash
mkdir -p /opt/aiclient2api/sing-box
cp /opt/sub2api/repo/deploy/a2-proxy/sing-box.config.json.example \
  /opt/aiclient2api/sing-box/config.json

# 编辑 /opt/aiclient2api/sing-box/config.json，把 outbounds 替换为真实代理节点
vim /opt/aiclient2api/sing-box/config.json

cd /opt/sub2api
cp /opt/sub2api/repo/deploy/docker-compose.a2-proxy.yml ./docker-compose.a2-proxy.yml
docker compose -f docker-compose.yml -f docker-compose.a2-proxy.yml up -d a2-proxy
```

然后在 AIClient2API Web UI 或 `/opt/aiclient2api/configs/config.json` 设置：

```json
{
  "PROXY_URL": "http://a2-proxy:10809",
  "PROXY_ENABLED_PROVIDERS": [
    "gemini-cli-oauth",
    "gemini-antigravity",
    "claude-kiro-oauth"
  ]
}
```

保存后重启 A2：

```bash
cd /opt/sub2api
docker compose up -d aiclient2api
```

验证代理出口：

```bash
docker run --rm --network sub2api_sub2api-network curlimages/curl:8.10.1 \
  -x http://a2-proxy:10809 https://api.ipify.org
```

注意：
- A2 在容器内运行，因此 `PROXY_URL` 不应填 `http://127.0.0.1:10809`；
  应填 Docker 内网服务名 `http://a2-proxy:10809`。
- `a2-proxy` 没有 `ports` 映射，只在 Docker 网络内可达，不对公网暴露。
- 占位配置是 direct-only，只用于先把服务框架部署好；真实节点信息到位后再替换 outbound。

关键点：
- AIClient2API 绑定到宿主机 `127.0.0.1:3000`，**不对公网暴露**
- Caddy（宿主机 systemd）从 `127.0.0.1:3000` 反代到公网子域名 `a2.zerocode.kaynlab.com`
- sub2api 的 gateway 转发仍走 Docker 内网 DNS `http://aiclient2api:3000/...`（绕过 Caddy 避免多余跳）

### AIClient2API Web UI 访问

**URL**: https://a2.zerocode.kaynlab.com
**登录密码**: 见下方 "生产口令存放位置"

密码强度为 32 字符 hex，不建议对外透露。Web UI 登录后可以：
- 添加/管理 Kiro 账号
- 查看 provider pool 健康状态
- 重新 OAuth 登录 Kiro
- 实时查看请求日志

### Caddyfile 配置

生产 `/etc/caddy/Caddyfile` 的 aiclient2api vhost 块（**不进 git**，手动维护）：

```
a2.zerocode.kaynlab.com {
	encode zstd gzip

	request_body {
		max_size 16MB
	}

	reverse_proxy 127.0.0.1:3000 {
		# Preserve SSE / streaming
		flush_interval -1
	}
}
```

Caddy 通过 `systemctl reload caddy` 热加载配置，不会中断现有连接。

### DNS 配置

Cloudflare 下 `a2.zerocode` A 记录指向 `172.245.247.80`，**proxied=false**（DNS Only），让 Caddy 直接拿 Let's Encrypt 证书。

### 部署命令

```bash
# 完整部署（sub2api + aiclient2api）
ssh -i ~/.ssh/id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh"

# 只部署 sub2api
bash /opt/sub2api/update.sh --skip-a2

# 只部署 aiclient2api
bash /opt/sub2api/update.sh --only-a2

# sub2api 回滚
bash /opt/sub2api/update.sh --rollback
```

### 首次部署步骤

1. 确认 `541968679/AIClient2API` 的 GitHub Actions 已成功发布 `ghcr.io/541968679/aiclient2api:latest`，并确保该 GHCR package 对生产服务器可拉取。
2. SSH 到生产服务器，准备 AIClient2API 持久化目录：
   ```bash
   mkdir -p /opt/aiclient2api/{configs,logs}
   ```
3. 本地 `scp` 上传 `deploy-staging/configs/` 到生产 `/opt/aiclient2api/configs/`
4. 本地推送 sub2api 改动：`git push origin main`
5. 执行部署：`bash /opt/sub2api/update.sh`
6. 在 sub2api 管理后台添加 anthropic API Key 账号：
   - API Key：`AICLIENT_API_KEY`（强口令）
   - Base URL：`http://aiclient2api:3000/claude-kiro-oauth`
   - 开启自动透传
7. 绑定到 anthropic 分组，用测试 key 验证模型响应

### Kiro 模型列表限制排查

生产服务器是美国 IP，理论上 OAuth 能拿到完整 Claude 模型。但如果上传的 token 是中国 IP 登录获得的，可能仍有限制。验证方法：
- 测试 `claude-opus-4-7`，若报 `INVALID_MODEL_ID`，token 被中国 IP 限制
- 备选：临时 `docker compose.override.yml` 暴露 aiclient2api:3000 到 22 端口，访问 Web UI 在美国 IP 环境下重成后移除 override

### 生产口令存放位置

口令**不进 git**。三处存放：

| 密钥 | 存放位置 | 用途 |
|---|---|---|
| `AICLIENT_API_KEY` | 本地 `E:\cursor project\AIClient2API\.env.production.secrets`（gitignored） | sub2api Anthropic 账号表单里的 API Key 字段 |
| `AICLIENT_API_KEY` | 生产 `/opt/aiclient2api/configs/config.json` 的 `REQUIRED_API_KEY` 字段 | AIClient2API 接收请求时的校验 |
| `AICLIENT_WEB_PASSWORD` | 本地 `E:\cursor project\AIClient2API\.env.production.secrets`（gitignored） | 记录用 |
| `AICLIENT_WEB_PASSWORD` | 生产 `/opt/aiclient2api/configs/pwd` 文件 | AIClient2API Web UI 登录密码 |

#### 查看生产口令

```bash
# 查看 API Key
ssh -i ~/.ssh/id_ed25519_sub2api root@172.245.247.80 \
  "grep REQUIRED_API_KEY /opt/aiclient2api/configs/config.json"

# 查看 Web UI 密码
ssh -i ~/.ssh/id_ed25519_sub2api root@172.245.247.80 \
  "cat /opt/aiclient2api/configs/pwd"
```

#### 轮换口令

1. 生成新口令：`openssl rand -hex 32`
2. 更新生产 `config.json`（`REQUIRED_API_KEY` 字段）或 `pwd` 文件
3. `docker restart aiclient2api`
4. 在 sub2api 后台同步更新对应账号的 API Key 字段
5. 本地 `.env.production.secrets` 同步更新

#### 本地开发口令

| 密钥 | 值 | 用途 |
|---|---|---|
| AIClient2API API Key | `123456` | 本地调试，不用于生产 |
| AIClient2API Web UI | `admin123` | 本地调试，不用于生产 |
