# Production Deployment

This repository tracks the Zhouyu Sub2API production deployment separately from upstream source code.

## Server

| Item | Value |
| --- | --- |
| Host | `root@47.100.224.163` |
| Local SSH key | `./sshkey.pem` |
| Deploy directory | `/opt/sub2api` |
| Compose file | `/opt/sub2api/docker-compose.yml` |
| Env file | `/opt/sub2api/.env` |
| Health check | `http://127.0.0.1:8080/health` |
| Inventory | `ops/servers/production.yml` |

Do not commit `sshkey.pem`, `.env`, generated passwords, API keys, or account tokens.
Initial generated credentials, when created by the deployment operator, are stored on the server at `/opt/sub2api/INITIAL_CREDENTIALS.txt` with mode `600`. Remove that file after the first successful login and password rotation.

## Version Model

The upstream source of truth is:

```text
https://github.com/Wei-Shaw/sub2api.git
```

Keep `upstream` pointed at the official repository. Use this flow for source sync:

```bash
git fetch upstream
git merge upstream/main
```

The current synced upstream release is `v0.2.0` (`weishaw/sub2api:0.2.0`). Source tree tracks `upstream/main`.

Production app images are built by GitHub Actions on the fork, not on a workstation and not on the server. Push the release line to `release/<version>` on `https://github.com/541968679/sub2api`. Do not push that line to the fork `main` branch. The workflow builds one `linux/amd64` image and pushes it to GHCR. It does not build arm64 and does not tag `:latest`.

```text
ghcr.io/541968679/sub2api:<version>
ghcr.io/541968679/sub2api:sha-<commit>
```

`<version>` is the branch name without the `release/` prefix. Pin `SUB2API_IMAGE` in `/opt/sub2api/.env` to the immutable `sha-<commit>` tag. The subscription-import image is `ghcr.io/541968679/sub2api:sha-83b387787b79ee4e82f6018e0a4abd72ca840dd1` (same image as `0.2.0-proxy-sub`). `weishaw/sub2api:0.2.0` remains the rollback image.

This repository started as a snapshot of upstream `54ef446c1` (VERSION `0.1.138`) with unrelated git history. After the first sync merge, later `git merge upstream/main` uses a normal merge-base. Keep the Zhouyu overlay under `deploy/production/`, `docs/ops/`, and `ops/servers/`.

## First Deploy

1. Copy the production files to the server:

```powershell
scp -i .\sshkey.pem .\deploy\production\docker-compose.yml root@47.100.224.163:/opt/sub2api/docker-compose.yml
scp -i .\sshkey.pem .\deploy\production\.env.example root@47.100.224.163:/opt/sub2api/.env.example
scp -i .\sshkey.pem .\deploy\production\update.sh root@47.100.224.163:/opt/sub2api/update.sh
scp -i .\sshkey.pem .\deploy\production\backup.sh root@47.100.224.163:/opt/sub2api/backup.sh
```

2. On the server, create `/opt/sub2api/.env` from `.env.example`, replace all placeholder secrets, and restrict permissions:

```bash
cd /opt/sub2api
cp .env.example .env
chmod 600 .env
chmod +x update.sh backup.sh
```

3. Deploy the full stack the first time:

```bash
bash /opt/sub2api/update.sh deploy-stack
```

If the server cannot pull Docker Hub images directly, install the mirror template:

```powershell
scp -i .\sshkey.pem .\deploy\production\docker-daemon.json.example root@47.100.224.163:/etc/docker/daemon.json
ssh -i .\sshkey.pem root@47.100.224.163 "systemctl restart docker"
```

## Routine Update

Use this path for every app release. The production host is Linux x86_64. GitHub Actions builds the matching `linux/amd64` image. The server only pulls that image and recreates the `sub2api` container. Leave Postgres and Redis running.

1. Push the release branch. Do not push it to fork `main`.

```powershell
git push origin HEAD:release/<version>
```

2. Wait until the `Build linux/amd64 image` workflow succeeds. Copy the `sha-<commit>` tag from that run.

3. Back up, then pin that tag in `/opt/sub2api/.env`. Change only the `SUB2API_IMAGE` line. Do not print the rest of the file.

4. Copy the updated app script when `deploy/production/update.sh` changed, then deploy:

```powershell
scp -i .\sshkey.pem .\deploy\production\update.sh root@47.100.224.163:/opt/sub2api/update.sh
ssh -i .\sshkey.pem root@47.100.224.163 "bash /opt/sub2api/update.sh deploy"
```

`deploy` pulls only `sub2api`, checks that the image is `linux/amd64`, and runs `docker compose up -d --no-deps sub2api`. It checks `http://127.0.0.1:18080/health` and `http://127.0.0.1:8080/health`.

Verify:

```bash
ssh -i .\sshkey.pem root@47.100.224.163 "bash /opt/sub2api/update.sh ps"
ssh -i .\sshkey.pem root@47.100.224.163 "curl -fsS http://127.0.0.1:18080/health"
ssh -i .\sshkey.pem root@47.100.224.163 "curl -fsS http://127.0.0.1:8080/health"
ssh -i .\sshkey.pem root@47.100.224.163 "docker inspect sub2api --format 'image={{.Config.Image}} status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}no-health{{end}}'"
```

Rollback: set `SUB2API_IMAGE` back to the previous image and run `update.sh deploy` again. The previous image for this host is `weishaw/sub2api:0.2.0`.

Before a large version jump, copy the updated compose file and set `SETUP_MIGRATION_TIMEOUT_SECONDS` high enough for database migrations:

```powershell
scp -i .\sshkey.pem .\deploy\production\docker-compose.yml root@47.100.224.163:/opt/sub2api/docker-compose.yml
```

Run `update.sh deploy-stack` only when the Postgres or Redis image also changes. That command pulls all three images and recreates the stack.

## Backup

Run before risky upgrades:

```bash
ssh -i .\sshkey.pem root@47.100.224.163 "bash /opt/sub2api/backup.sh"
```

Backups are written under `/opt/sub2api/backups`.

## Network Exposure

The container publishes `0.0.0.0:8080` by default. If `curl http://127.0.0.1:8080/health` works on the server but `http://47.100.224.163:8080/health` is unreachable externally, check the cloud security group first. On the server, UFW should not need changes unless it is explicitly enabled.
