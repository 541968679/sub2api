# Kiro Gateway 附加项目

> Kiro Gateway 是 Sub2API 的独立 sidecar，不合并进本仓库代码。它负责把本机
> Kiro 账号反向代理成 Anthropic/OpenAI 兼容 API，再由 Sub2API 作为上游 API
> 账号接入。

## 数据模型

| 实体/字段 | 位置 | 说明 |
|-----------|------|------|
| Kiro Gateway 仓库 | `E:\cursor project\kiro-gateway` | 独立 Git 仓库，来源 `https://github.com/Jwadow/kiro-gateway` |
| Kiro Gateway `.env` | `E:\cursor project\kiro-gateway\.env` | 本地监听地址、端口、代理 API Key、账号系统开关 |
| Kiro Gateway `credentials.json` | `E:\cursor project\kiro-gateway\credentials.json` | Kiro 账号配置；启动时必须至少有一个可初始化账号 |
| Sub2API Anthropic API Key 账号 | Sub2API 管理后台账号页 | 用 Kiro Gateway 的 Anthropic `/v1/messages` 兼容接口作为上游 |

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| Sidecar | `E:\cursor project\kiro-gateway\README.md` | Kiro Gateway 官方启动和配置说明 |
| Sidecar | `E:\cursor project\kiro-gateway\main.py` | FastAPI 入口，启动时校验 Kiro 凭据并初始化账号 |
| Sidecar | `E:\cursor project\kiro-gateway\credentials.json.example` | 多账号配置示例 |
| Sidecar | `E:\cursor project\kiro-gateway\kiro\routes_anthropic.py` | Anthropic `/v1/messages` 兼容入口 |
| Sub2API | `backend/internal/server/routes/gateway.go` | `/v1/chat/completions` 按分组平台路由；非 OpenAI 分组走 Anthropic compatibility |
| Sub2API | `backend/internal/service/gateway_forward_as_chat_completions.go` | Chat Completions -> Anthropic Messages 转换 |
| Sub2API | `backend/internal/service/gateway_service.go` | Anthropic API Key 上游请求构造；自定义 base URL 会拼接 `/v1/messages?beta=true` |

## 核心流程

### 本地 sidecar 启动

```powershell
cd "E:\cursor project\kiro-gateway"
.\.venv\Scripts\python.exe main.py
```

当前本地配置：

```env
PROXY_API_KEY="sub2api-kiro-local-dev"
SERVER_HOST="127.0.0.1"
SERVER_PORT="8000"
ACCOUNT_SYSTEM="true"
ACCOUNTS_CONFIG_FILE="credentials.json"
ACCOUNTS_STATE_FILE="state.json"
```

### Sub2API 接入

在 Sub2API 管理后台添加账号：

| 字段 | 推荐值 |
|------|--------|
| 平台 | Anthropic |
| 类型 | API Key |
| Base URL | `http://127.0.0.1:8000` |
| API Key | `sub2api-kiro-local-dev` |

请求链路：

```
Client
  -> Sub2API /v1/messages 或 /v1/chat/completions
  -> Anthropic 分组/账号调度
  -> Kiro Gateway http://127.0.0.1:8000/v1/messages
  -> Kiro 账号
```

## 重要机制

| 机制 | 说明 |
|------|------|
| API 格式优先级 | 优先按 Anthropic API Key 账号接入，而不是 OpenAI 账号。Sub2API 的 Anthropic compatibility 路径会稳定调用 `/v1/messages`。 |
| Chat Completions 兼容 | 客户端仍可请求 Sub2API `/v1/chat/completions`；Sub2API 会转换到 Anthropic Messages 后调用 Kiro Gateway。 |
| 启动前置条件 | Kiro Gateway 启动时必须初始化至少一个有效 Kiro 账号。空 `credentials.json` 或过期 token 会导致服务退出。 |
| Windows 编码 | 直接前台运行时需使用 UTF-8 环境，避免启动 banner emoji 在 GBK 控制台触发 `UnicodeEncodeError`。 |

## 已知陷阱

- 本机检测到 `C:\Users\mechrev-kayn\.aws\sso\cache\kiro-auth-token.json`，但 Kiro Gateway 刷新该 token 时返回 401。需要先重新登录 Kiro IDE/CLI，或手动把新的账号凭据写入 `credentials.json`。
- Kiro Gateway 不是运行后在 Web UI 添加账号；账号管理通过 `credentials.json` 或 `.env` 完成，修改后重启服务。
- `PROXY_API_KEY` 是保护本地 Kiro Gateway 的自定义密码，不是 Kiro 官方 token。Sub2API 里填写的 API Key 应与它一致。
- 如端口 8000 被占用，可改 `.env` 的 `SERVER_PORT` 或用 `python main.py --port 9000`，Sub2API 的 Base URL 也要同步改端口。
