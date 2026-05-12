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

## 未完成：antigravity 分组接入 Kiro（遗留问题）

### 需求

客户已有大量 antigravity 分组的 API Key 在用，希望不改 URL（继续用 `http://localhost:5175/antigravity/v1/messages`）就能切换到 Kiro 上游。

### 当前状态

已实现的方案 A：**路由层回退**
- Kiro 账号保持在 anthropic 平台下
- `selectAccount` 在 antigravity 平台找不到可用账号时，回退查找 anthropic passthrough 账号
- 代码位置：`gateway_service.go` 第 1411 行附近

### 遗留问题

1. **方案 A 实测仍报 `claude-opus-4-7[1m]` 模型错误** — 怀疑是 Go 代码未正确编译或请求走了其他未清理模型名的路径。需要下次抓日志定位。
2. **antigravity 分组的 API Key 无法在 sub2 平台获取额度信息** — 同一个 key 在 anthropic 分组下能正常显示余额，antigravity 分组下显示不出来。可能是 billing 查询端点对 platform 做了限制。
3. **API 调用速度偏慢** — 尚未做网络链路分析。可能的瓶颈：sub2api → AIClient2API（localhost 本应很快）、AIClient2API → Kiro（走代理，国外往返）。

### 下次排查方向

1. 打开 sub2api 后端 debug 日志，抓一个完整 antigravity 分组请求链路
2. 检查 `selectAccountForModelWithPlatform` 是否真的返回 error 触发回退
3. 检查 count_tokens 路径（流式请求前置）是否也走了 passthrough
4. 确认 `strings.Index` 清理 `[1m]` 逻辑在流式路径生效
5. 额度接口：grep `balance` / `usage` / `quota` 相关 handler，看 antigravity platform 的区别处理

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
| `deploy/update.sh` | 扩展为同时构建 aiclient2api sidecar，支持 --skip-a2 / --only-a2 |
| `E:\cursor project\AIClient2API\src\providers\claude\claude-kiro.js` | 修改身份注入为动态 `${model}` |

## 生产环境部署

### 仓库 Fork

| 上游 | Fork | 角色 |
|---|---|---|
| `Wei-Shaw/sub2api` | `541968679/sub2api` | sub2api 主项目 |
| `justlovemaki/AIClient2API` | `541968679/AIClient2API` | Kiro 反代 sidecar |

### 构建方式

**不使用 GitHub CI**。两个项目都在生产服务器上就地构建（和 sub2api 原有流程一致）：

```
git pull → docker build --no-cache → docker tag → docker compose up -d
```

### 生产目录结构

```
/opt/sub2api/
├── repo/                          # sub2api 源码 (git clone)
├── aiclient2api-repo/             # AIClient2API 源码 (git clone fork)
├── docker-compose.yml             # 主 compose（包含两个 service）
├── update.sh                      # 部署脚本，默认同时构建两者
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
   ↓
Caddy :443 (auto HTTPS)
   ↓
sub2api :8080 (docker network)
   ↓ http://aiclient2api:3000/claude-kiro-oauth/v1/messages
aiclient2api :3000 (docker network, NOT exposed to public)
   ↓ https://q.us-east-1.amazonaws.comS server IP)
Kiro API
```

AIClient2API 不对公网暴露（compose 里没配 `ports`），只有 sub2api 能通过 docker 内网 DNS 访问。

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

1. SSH 到生产服务器，准备 AIClient2API 目录：
   ```bash
   git clone https://github.com/541968679/AIClient2API.git /opt/sub2api/aiclient2api-repo
   mkdir -p /opt/aiclient2api/{configs,logs}
   ```
2. 本地 `scp` 上传 `deploy-staging/configs/` 到生产 `/opt/aiclient2api/configs/`
3. 本地推送 sub2api 改动：`git push origin main`
4. 执行部署：`bash /opt/sub2api/update.sh`
5. 在 sub2api 管理后台添加 anthropic API Key 账号：
   - API Key：`AICLIENT_API_KEY`（强口令）
   - Base URL：`http://aiclient2api:3000/claude-kiro-oauth`
   - 开启自动透传
6. 绑定到 anthropic 分组，用测试 key 验证模型响应

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
