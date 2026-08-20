# Sub2API 部署运维手册

## 一、部署架构概览

```
用户请求 → Caddy/Nginx (反代+SSL) → Sub2API (:8080) → 上游 AI API
                                        ↓
                                   PostgreSQL + Redis
```

### 1.1 本项目生产环境速查

线上排查 / 生产报错 / 拉日志：先读 `docs/dev/PRODUCTION.md`，先拉日志再看代码。本节继续管部署入口与发布历史。

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

Sub2API 主服务必须使用 GitHub Actions 发布到 GHCR 的镜像。不要在生产服务器执行 `docker build`，不要部署 `sub2api-custom:*`。当前 `deploy/update.sh` 主服务路径：写入 GHCR compose override → `docker compose pull sub2api` → **用 `sub2api-preflight` 侧车跑通 InitEnt/`/health`（此时线上 `sub2api` 继续服务）** → 预检通过后才 `force-recreate`。预检失败则中止，旧容器不动。生产使用前先把本仓库的脚本同步到 `/opt/sub2api/update.sh`。

**禁止再部署 `v0.1.232` 或 `v0.1.233`。** `v0.1.232` 在启动时被迁移校验器拒绝（`205_usage_log_true_cost.sql` 注释含 `CONCURRENTLY`），旧脚本会先 `force-recreate` 杀掉健康容器。`v0.1.233` 预检失败：206 注释里的分号被当成 SQL 执行。下一版必须是新版本号（`0.1.234+`），不要复用这两个 tag。

```powershell
# 只部署 Sub2API 主服务（GHCR）
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai"

# 只部署 AIClient2API 侧车
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh --only-a2"

# 只部署 InvokeAI 侧车
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh --only-invokeai"

# 完整部署 Sub2API + AIClient2API + InvokeAI（GHCR；执行前确认三者镜像均可 pull）
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh"
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
- **禁止部署 `v0.1.232` / `v0.1.233`**（232：205 注释含 `CONCURRENTLY` crash-loop；233：206 注释分号被 Exec 为 SQL）。下一版必须是 `0.1.234+`。`update.sh` 会拒绝这两个 tag，且必须先预检再 recreate。
- 当前 Release workflow 只会在 `v*` tag 推送或手动 `workflow_dispatch` 时发布 GHCR 镜像；单独 push `main` 不会刷新 `ghcr.io/541968679/sub2api:latest`。生产 `pull/up` 前必须确认目标 tag 或 `latest` 已经存在，并且镜像 label 指向本次要部署的 commit。
- `deploy/update.sh` 会替换由旧脚本生成的 `sub2api-custom:latest` override；遇到无法识别的自定义 override 会拒绝覆盖。部署前后都要确认 `docker compose config` 解析出的 `sub2api.image` 是 `ghcr.io/541968679/sub2api:latest` 或本次明确批准的 GHCR tag/digest。
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
`workflow_dispatch`。fork `541968679/sub2api` 的仓库变量 `SIMPLE_RELEASE=true`
（也可在 `workflow_dispatch` 勾选 `simple_release`）。simple 配置
`.goreleaser.simple.yaml` 只编 linux/amd64，发布
`ghcr.io/541968679/sub2api:<version>`、`:<version>-amd64` 和 `:latest`
（三个标签指向同一张 amd64 镜像，不是 multi-arch manifest）。
关掉该变量才会走完整 `.goreleaser.yaml`（Windows / Darwin / linux-arm64 + QEMU）。

主服务生产部署按这个顺序执行：

1. 将已经验证的代码合入并推送到 `main`。
2. 创建并推送下一个 `v*` tag，或手动触发 Release workflow。
3. 等 GitHub Actions Release 成功后，确认 GHCR 镜像已经发布。
4. 确认生产 Compose 的 `sub2api.image` 指向 GHCR，而不是
   `sub2api-custom:*`。
5. 将本仓库 `deploy/update.sh` 同步到生产机 `/opt/sub2api/update.sh`，然后执行
   `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`。脚本会先预检新镜像；
   预检失败时旧容器继续服务，不要靠拉长 `HEALTH_RETRIES` 硬等 crash-loop。
6. 核对运行镜像、revision/version label、容器健康状态和 `/health`。

#### 主服务预检（2026-08-17 起）

`v0.1.232` 事故：`update.sh` 对 `sub2api` 执行 `docker compose up -d --no-deps --force-recreate`，旧进程先停，新进程在 `InitEnt` 校验迁移 205 时退出。临时把 `HEALTH_RETRIES=60` / `INTERVAL=10` 只会让站点多停几分钟。

现行规则：

1. `docker compose pull sub2api`（旧容器仍在 `127.0.0.1:8080` 服务）。
2. `docker compose run -d --no-deps --name sub2api-preflight sub2api`：同一 compose 网络/环境/数据卷，**不**加 `--service-ports`，不占用宿主机 8080。
3. 在预检容器内 `wget http://127.0.0.1:8080/health`。进程若因迁移校验/`InitEnt` 退出，立即中止并 `docker rm -f sub2api-preflight`。
4. 预检通过后才 `force-recreate` 线上 `sub2api`。线上健康检查保持短重试（默认 5×5s）；长等待只用于预检（默认 36×5s）。
5. 预检失败：**不** recreate、**不**切流量，并把 `docker-compose.override.yml` 写回旧 digest，避免之后误 `compose up` 切到坏镜像。自动 rollback 只处理「预检已过、切换后仍失败」的情况，不能替代预检。

残留风险：预检会走完整启动（含 `206` 的 `CREATE INDEX CONCURRENTLY`）。校验通过但索引构建很久时，预检会等到超时；超时会杀掉预检容器，可能留下 `INVALID` 索引。206 启动前会 `DROP INDEX CONCURRENTLY` 无效索引后再建。这段时间线上旧容器仍在服务。双实例短窗口内会共用 Redis/数据卷，预检一过 `/health` 立即拆掉。

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
| 2026-08-20 | `v0.1.245` | `872951282` | `ghcr.io/541968679/sub2api:0.1.245` | `0.1.245` | **current**; digest `sha256:17f79f1e15cab372386fa3237a6974fa2337c6a3bb05760b64f8da5851e13ef3`, `/health` ok, healthy; pair-full no WaitPlan + wait-wake pair attach; quality cell failover/Recovered comparison success rate (display only); rollback digest `sha256:1cae09a00e3b35f91fcdf34726bb2c3d0b9f78a246ef796689ecc948ec79edb7` (`v0.1.244`) |
| 2026-08-20 | `v0.1.244` | `91adbd539` | `ghcr.io/541968679/sub2api:0.1.244` | `0.1.244` | superseded by `v0.1.245`; digest `sha256:1cae09a00e3b35f91fcdf34726bb2c3d0b9f78a246ef796689ecc948ec79edb7`, `/health` ok, healthy; passthrough OpenAI routing + sync inbound upstream SSE buffer (no client stream change); rollback digest `sha256:d7e97bb50f8d2ad5053d758e6a344c04f992dcdd897ad1790bb1c8d9ad6733f7` (`v0.1.243`) |
| 2026-08-20 | `v0.1.243` | `09d567545` | `ghcr.io/541968679/sub2api:0.1.243` | `0.1.243` | superseded by `v0.1.244`; digest `sha256:d7e97bb50f8d2ad5053d758e6a344c04f992dcdd897ad1790bb1c8d9ad6733f7`, `/health` ok, healthy; delete-account detaches smart-schedule pool ghosts; rollback digest `sha256:785c5295521823154e7aa8e147fe8e4d57c36d320f23ccbb849013544b8841fb` (`v0.1.242`) |
| 2026-08-19 | `v0.1.242` | `a9761738a` | `ghcr.io/541968679/sub2api:0.1.242` | `0.1.242` | superseded by `v0.1.243`; digest `sha256:785c5295521823154e7aa8e147fe8e4d57c36d320f23ccbb849013544b8841fb`, `/health` ok, healthy; NewAPI slim completed default-off + user 220 whitelist (`openai_newapi_slim_completed=false`, `user_ids=[220]`); rollback digest `sha256:8fc8335b63490ceedf07991bd47004014812a8e8a54159636d8b482acd1bd90b` (`v0.1.241`) |
| 2026-08-19 | `v0.1.241` | `c8d497732` | `ghcr.io/541968679/sub2api:0.1.241` | `0.1.241` | superseded by `v0.1.242`; digest `sha256:8fc8335b63490ceedf07991bd47004014812a8e8a54159636d8b482acd1bd90b`, `/health` ok, healthy; OpenAI update-RT session JSON + smart-schedule interval auto-sort + OAuth 7-day quota PnL; rollback digest `sha256:ba60309580077e97fcd3e14313e57f2c2d5018d0662ad41ba1421148a9389221` (`v0.1.240`) |
| 2026-08-18 | `v0.1.240` | `e5608cdbe` | `ghcr.io/541968679/sub2api:latest` (`0.1.240`) | `0.1.240` | superseded by `v0.1.241`; digest `sha256:ba60309580077e97fcd3e14313e57f2c2d5018d0662ad41ba1421148a9389221`, `/health` ok, healthy; CC→Responses fail-fast + API Key non-stream JSON; rollback digest `sha256:fb45fbf77ad49595f956c852d4fe8a5bba37a68f558f86587ccef2b7140ae362` (`v0.1.239`) |
| 2026-08-18 | `v0.1.239` | `4394f74b1` | `ghcr.io/541968679/sub2api:latest` (`0.1.239`) | `0.1.239` | superseded by `v0.1.240`; digest `sha256:fb45fbf77ad49595f956c852d4fe8a5bba37a68f558f86587ccef2b7140ae362`, `/health` ok, healthy; restore user TTFT/success after dual-caliber SQL 500 (`FILTER (WHERE FALSE)`); rollback digest `sha256:886d1444c76c186c215dc8434408a3cc60e151b293c46f8bb431c500c300a1c1` (`v0.1.238`) |
| 2026-08-18 | `v0.1.238` | `bbb3c159d` | `ghcr.io/541968679/sub2api:latest` (`0.1.238`) | `0.1.238` | superseded by `v0.1.239`; digest `sha256:886d1444c76c186c215dc8434408a3cc60e151b293c46f8bb431c500c300a1c1`, `/health` ok, healthy; dual account error-rate calibers + default-off failover schedule toggle + Recovered list badges; `v0.1.237` tag exists but GHCR image was never published (vue-tsc TS2783); rollback digest `sha256:926dd9aee523a4c9ba3abc681caa4b29871f1f9da870db5ddde72347aea9da71` (`v0.1.236`) |
| 2026-08-17 | `v0.1.236` | `8d0fc0ea5` | `ghcr.io/541968679/sub2api:latest` (`0.1.236`) | `0.1.236` | superseded by `v0.1.238`; digest `sha256:926dd9aee523a4c9ba3abc681caa4b29871f1f9da870db5ddde72347aea9da71`, `/health` ok, healthy; smart-schedule pool today PnL + upstream balance + column settings toolbar; rollback digest `sha256:f3bed8ed271032ac2bc73acecfe367435beebb25844fbaf6e8b180b6c10d3716` (`v0.1.235`) |
| 2026-08-17 | `v0.1.235` | `c439daec0` | `ghcr.io/541968679/sub2api:0.1.235` | `0.1.235` | superseded by `v0.1.236`; digest `sha256:f3bed8ed271032ac2bc73acecfe367435beebb25844fbaf6e8b180b6c10d3716`, `/health` ok, healthy; content_part harvest + incident docs + schedule excludes Claude-GPT bridge errors; rollback digest `sha256:55958f1a21bb0ec7e088e1c47ec20504f282274644f73da10883fe9077a7f65e` (`v0.1.234`) |
| 2026-08-17 | `v0.1.234` | `c1b335974` | `ghcr.io/541968679/sub2api:0.1.234` | `0.1.234` | superseded by `v0.1.235`; digest `sha256:55958f1a21bb0ec7e088e1c47ec20504f282274644f73da10883fe9077a7f65e`, `/health` ok, healthy; migrate comment-split fix; 205 already applied by 233 preflight, 206 applied; rollback digest `sha256:4954c27bc7c0764b9a665526020742ab22ea03fa3c25a66c1570a43efd1d8a61` (`v0.1.231`) |
| 2026-08-17 | `v0.1.233` | (banned) | `ghcr.io/541968679/sub2api:0.1.233` | `0.1.233` | **BANNED — never redeploy.** Preflight failed: 206 comment semicolon was Exec'd as SQL. Live stayed on `v0.1.231`. 205 was applied by that preflight. |
| 2026-08-17 | `v0.1.232` | (banned) | `ghcr.io/541968679/sub2api:0.1.232` | `0.1.232` | **BANNED — never redeploy.** Crash-loop: `validate migration 205_usage_log_true_cost.sql: CONCURRENTLY statements must be placed in *_notx.sql migrations` (comment-only token). 205/206 were not applied. Rolled back to `v0.1.231`. Do not reuse. |
| 2026-08-17 | `v0.1.231` | `f9a701878` | `ghcr.io/541968679/sub2api:0.1.231` | `0.1.231` | superseded by `v0.1.234`; rollback digest `sha256:4954c27bc7c0764b9a665526020742ab22ea03fa3c25a66c1570a43efd1d8a61`, `/health` ok, healthy; uncapped pair occupancy writes + live pool 调度优先级; `lb_top_k` unchanged |
| 2026-08-17 | `v0.1.230` | `d3cdb8cab` | `ghcr.io/541968679/sub2api:0.1.230` | `0.1.230` | superseded by `v0.1.231`; rollback digest `sha256:27ae51ce035c574873ee174a9853d2bc4e83f2e71a815769d80939ab96b5c1ad`; pool `sort_order` + uncapped pair `n/999`; `lb_top_k` unchanged |
| 2026-08-16 | `v0.1.229` | `838d370a7` | `ghcr.io/541968679/sub2api:0.1.229` | `0.1.229` | superseded by `v0.1.230`; rollback digest `sha256:7037fcacc6603e380ba9980c04c30486164166e09b80e8be14b6051c5f24f541`; smart-schedule UX batch + pool-mode hard eviction |
| 2026-08-16 | `v0.1.228` | `409e37c83` | `ghcr.io/541968679/sub2api:0.1.228` | `0.1.228` | superseded by `v0.1.229`; rollback digest `sha256:aa32950bf9b1a0eccfe3578f00ca3c8cbe418a83585c1edb0385b1ddb6d21634`; smart-schedule pool admin UX + stability chart |
| 2026-08-16 | `v0.1.227` | `264a6cda7` | `ghcr.io/541968679/sub2api:0.1.227` | `0.1.227` | superseded by `v0.1.228`; user smart schedule pools + upstream rate overlay |
| 2026-08-15 | `v0.1.226` | `a9beee087` | `ghcr.io/541968679/sub2api:0.1.226` | `0.1.226` | superseded by `v0.1.227`; Codex compact V2 synthetic `encrypted_content` |
| 2026-08-15 | `v0.1.225` | `be9a3208f` | `ghcr.io/541968679/sub2api:0.1.225` | `0.1.225` | superseded by `v0.1.226`; Codex remote compact V2 API-key fallback (default on) |
| 2026-08-15 | `v0.1.224` | `77647f9c1` | `ghcr.io/541968679/sub2api:0.1.224` | `0.1.224` | superseded by `v0.1.225`; admin user pin-to-top + live account quality chips |
| 2026-08-15 | `v0.1.223` | `95d9556a4` | `ghcr.io/541968679/sub2api:0.1.223` | `0.1.223` | superseded by `v0.1.224`; quality chips + two-step resume, merged schedule/capacity/quality columns |
| 2026-08-14 | `v0.1.222` | `85c02baee` | `ghcr.io/541968679/sub2api:0.1.222` | `0.1.222` | superseded by `v0.1.223`; quality history + opt-in hard-close, per-user quality gates, durable resume HASH |
| 2026-08-14 | `v0.1.221` | `1fbe4f2c1` | `ghcr.io/541968679/sub2api:0.1.221` | `0.1.221` | superseded by `v0.1.222`; independent allow/deny lists + per-user pair concurrency |
| 2026-08-14 | `v0.1.220` | `1baa78ce9` | `ghcr.io/541968679/sub2api:0.1.220` | `0.1.220` | superseded by `v0.1.221`; page auto-refresh keeps polling in background tabs |
| 2026-08-13 | `v0.1.219` | `9f006ade4` | `ghcr.io/541968679/sub2api:0.1.219` | `0.1.219` | superseded by `v0.1.220`; user-list TTFT/success-rate, account user allow/deny schedule, usage/error inspect dialog |
| 2026-08-13 | `v0.1.218` | `80f801fd9` | `ghcr.io/541968679/sub2api:0.1.218` | `0.1.218` | superseded by `v0.1.219`; display vs true first-token + preamble-flush user allowlist (global still off) |
| 2026-08-13 | `v0.1.217` | `824421852` | `ghcr.io/541968679/sub2api:0.1.217` | `0.1.217` | superseded by `v0.1.218`; native Responses first_token_ms on first SSE + admin `openai_responses_flush_preamble` (default off) |
| 2026-08-12 | `v0.1.216` | `761dd8cf9` | `ghcr.io/541968679/sub2api:0.1.216` | `0.1.216` | superseded by `v0.1.217`; restore admin usage `token`/`TOKEN` headers + per-column cache share |
| 2026-08-12 | `v0.1.211` | `60f971b8a` | `ghcr.io/541968679/sub2api:0.1.211` (override pin) | `0.1.211` | running, healthy, internal `/health` OK; admin display_fields L1+L2 + B1 cache amplify under M; **still skips banned v0.1.199** |
| 2026-08-12 | `v0.1.210` | `1a1786cb5` | `ghcr.io/541968679/sub2api:0.1.210` (override pin) | `0.1.210` | superseded by `v0.1.211`; admin usage display tokens column; **still skips banned v0.1.199** |
| 2026-08-11 | `v0.1.209` | `cc261b5d5` | `ghcr.io/541968679/sub2api:0.1.209` (override pin) | `0.1.209` | superseded by `v0.1.210`; ops logs real OpenAI `/v1/responses` for Claude-GPT bridge; **still skips banned v0.1.199** |
| 2026-08-11 | `v0.1.208` | `07d72e113` | `ghcr.io/541968679/sub2api:0.1.208` (override pin) | `0.1.208` | superseded by `v0.1.209`; Claude→GPT bridge PTL/compact lifecycle INFO logs; **still skips banned v0.1.199** |
| 2026-08-11 | `v0.1.207` | `098dbaefa` | `ghcr.io/541968679/sub2api:0.1.207` (override pin) | `0.1.207` | superseded by `v0.1.208`; admin OpenAI advanced scheduler weight override save persistence; **still skips banned v0.1.199** |
| 2026-08-10 | `v0.1.206` | `2b4fa84a0` | `ghcr.io/541968679/sub2api:0.1.206` (override pin) | `0.1.206` | superseded by `v0.1.207`; admin user balance burn-rate column (opt-in, 5m window, 15s poll, $/h|/min); **still skips banned v0.1.199** |
| 2026-08-10 | `v0.1.205` | `0bf08d3f4` | `ghcr.io/541968679/sub2api:0.1.205` (override pin) | `0.1.205` | superseded by `v0.1.206`; page-local account drag reorder + Codex image generation rendering + upstream-sync-guard protection; **still skips banned v0.1.199** |
| 2026-08-09 | `v0.1.204` | `841b80ad7` | `ghcr.io/541968679/sub2api:0.1.204` (override pin) | `0.1.204` | superseded by `v0.1.205`; display-balance save fix, create/edit/bulk 3-zone layout, list pin on create, move-to-top beside checkbox, wider dialogs; **still skips banned v0.1.199** |
| 2026-08-08 | `v0.1.203` | `4cc392a43` | `ghcr.io/541968679/sub2api:0.1.203` (override pin) | `0.1.203` | superseded by `v0.1.204`; display-only balance used/total + auto refresh; **still skips banned v0.1.199** |
| 2026-08-07 | `v0.1.202` | `c79442f0c` | `ghcr.io/541968679/sub2api:0.1.202` (override pin) | `0.1.202` | superseded by `v0.1.203`; per-account `model_mapping_strict_scheduling` + admin usage error deep-link UX; **still skips banned v0.1.199** |
| 2026-08-07 | `v0.1.201` | `c733bea81` | `ghcr.io/541968679/sub2api:0.1.201` (override pin) | `0.1.201` | superseded by `v0.1.202`; clear-concurrency menu + stream debug logs on 200 baseline; **still skips banned v0.1.199** sticky-escape scheduling changes |
| 2026-08-07 | `v0.1.201` | `c733bea81` | `ghcr.io/541968679/sub2api:0.1.201` (override pin) | `0.1.201` | running, healthy, internal `/health` OK; clear-concurrency menu + stream debug logs on 200 baseline; **still skips banned v0.1.199** sticky-escape scheduling changes |
| 2026-08-07 | `v0.1.200` | `97dba0a1a` | `ghcr.io/541968679/sub2api:0.1.200` (override pin) | `0.1.200` | superseded by `v0.1.201`; 198 baseline + admin recharge-history manage/delete; **skips banned v0.1.199** sticky/concurrency scheduling changes |
| 2026-08-07 | `v0.1.198` | (rollback pin) | `ghcr.io/541968679/sub2api:0.1.198` | `0.1.198` | emergency rollback from v0.1.199; superseded by `v0.1.200` |
| 2026-08-03 | `v0.1.197` | `734787d60` | `ghcr.io/541968679/sub2api:latest` | `0.1.197` | running, healthy, digest `sha256:ceb2c59c869e7bcfb07c0ea9e9dd5485030fa41ff9ec558ec3395a099c8581fd`, internal `/health` OK; account move-to-top + mobile user columns + concurrency sort; includes New API balance (`v0.1.196`) |
| 2026-08-12 | `v0.1.215` | `9ffb6f7ee` | `ghcr.io/541968679/sub2api:0.1.215` | `0.1.215` | running, healthy, internal `/health` OK; sampled OpenAI stream_stage timing (prod enabled for account 1689, sample_rate=1.0) |
| 2026-08-12 | `v0.1.214` | `397581932` | `ghcr.io/541968679/sub2api:0.1.214` | `0.1.214` | running, healthy, digest `sha256:814fe7cfcb979445e235275b2a7ad795865618f2636ad1498aafa57a97a62efd`, internal `/health` OK; local-main restore + usage heavy-user aggregates |
| 2026-08-12 | `v0.1.213` | `37cbeaa96` | `ghcr.io/541968679/sub2api:0.1.213` | `0.1.213` | running, healthy, digest `sha256:4bfdbbcf7a59e6fdfe0935fa61b4dec74f7956f57bb30b4d6dc85515f76e3dbb`, internal `/health` OK; usage heavy-user aggregates + gateway first_token_ms + admin token columns |
| 2026-08-07 | `v0.1.199` | `b0187a7f0` | `ghcr.io/541968679/sub2api:latest` | `0.1.199` | running, healthy, digest `sha256:67f8172cc6126659521b2fc4eb9735ae7bf51841f2de9e1ef9cb9963fe0e3420`, internal `/health` OK; sticky escape deletes binding; disable/bulk-disable clears sticky+concurrency; admin clear-stuck-runtime menu |
| 2026-08-06 | `v0.1.198` | `6b0263cee` | `ghcr.io/541968679/sub2api:latest` | `0.1.198` | superseded by `v0.1.199`; account edit group-checkbox fix + account column order/resize + users list auto-refresh |
| 2026-08-03 | `v0.1.197` | `734787d60` | `ghcr.io/541968679/sub2api:latest` | `0.1.197` | superseded by `v0.1.198`; account move-to-top + mobile user columns + concurrency sort; includes New API balance (`v0.1.196`) |
| 2026-08-03 | `v0.1.195` | `f588f50ab` | `ghcr.io/541968679/sub2api:latest` | `0.1.195` | superseded by `v0.1.197`; API key balance probe + burn-rate ETA (Sub2API `/v1/usage`) |
| 2026-08-02 | `v0.1.194` | `48787ba6a` | `ghcr.io/541968679/sub2api:latest` | `0.1.194` | superseded by `v0.1.195`; OAI fleet used/capacity + two-column layout |
| 2026-08-02 | `v0.1.193` | `a7b37e0c0` | `ghcr.io/541968679/sub2api:latest` | `0.1.193` | superseded by `v0.1.194`; OAI Pro/Prolite fleet 5h/7d used badge |
| 2026-08-02 | `v0.1.192` | `ab308b741` | `ghcr.io/541968679/sub2api:latest` | `0.1.192` | superseded by `v0.1.193`; group capacity group-scoped used; user list view-usage deep link |
| 2026-08-02 | `v0.1.191` | `3fefec077` | `ghcr.io/541968679/sub2api:latest` | `0.1.191` | superseded by `v0.1.192`; fix subscription usage metrics current-term scope + rate cap 100% |
| 2026-08-02 | `v0.1.190` | `a7bf77b73` | `ghcr.io/541968679/sub2api:latest` | `0.1.190` | superseded by `v0.1.191`; subscription group rates/usage columns/sort, usage recent filters |
| 2026-08-01 | `v0.1.189` | `b1e22ded6` (tag tree; latest main admin UX batch) | `ghcr.io/541968679/sub2api:latest` | `0.1.189` | running, healthy, digest `sha256:204a3e594425800f10213886d4d7aa88d4b96c257a25aafc4b61574f76356d0d`, internal `/health` OK; redeem batch UI, account inline edits, TTFT p50/p95, filter memory, group/account layout |
| 2026-08-01 | `v0.1.187` | `7a7cf372b8ace3ee388dfc9ccb01495fc8072488` | `ghcr.io/541968679/sub2api:latest` | `0.1.187` | running, healthy, digest `sha256:9498746a7d4d5150e9e6ae1899cb476e477ca5b14313d96177cc8add6cccfab0`, internal `/health` OK; inline redeem + redeem buy notice |
| 2026-07-30 | `v0.1.182` | `bf21c543a2823451b4de397585833d8ffb716519` | `ghcr.io/541968679/sub2api:0.1.182` | `0.1.182` | running, healthy, digest `sha256:1cd517da8ede857548f44c92bab62e850071318961d5afe83fa6be09cd20ac83`, internal `/health` OK; purchase-page notice emergency deploy |
| 2026-07-25 | `v0.1.175` | `80d9fd818ed248458772335df802fd691f6db6e5` | `ghcr.io/541968679/sub2api:latest` | `0.1.175` | running, healthy, restart count `0`, digest `sha256:9122cd929b70eb99fdef46f495d3cf178bbd858f35f35a39d85e02351642a38d`, internal/public `/health` OK |
| 2026-07-25 | `v0.1.174` | `fc543d1503b06a2b4c2e2eddacfcfc5ea41fc96e` | `ghcr.io/541968679/sub2api:latest` | `0.1.174` | running, healthy, restart count `0`, internal/public `/health` OK, migration `192` applied |
| 2026-07-24 | `v0.1.173` | `8ca41688ff7e61d75c0cefe2401231cfb5f6eb22` | `ghcr.io/541968679/sub2api:latest` | `0.1.173` | running, healthy, `/health` OK |
| 2026-07-23 | `v0.1.172` | `e5754a80d7ed43c0fc6a756f9b1eccef7283dd0f` | `ghcr.io/541968679/sub2api:latest` | `0.1.172` | running, healthy, `/health` OK |
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
