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

The server should run a published Docker image. For planned upgrades, prefer pinning `SUB2API_IMAGE` to a concrete upstream version tag in `/opt/sub2api/.env`. Use `weishaw/sub2api:latest` only when intentionally tracking the moving latest image.

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

3. Deploy:

```bash
bash /opt/sub2api/update.sh deploy
```

If the server cannot pull Docker Hub images directly, install the mirror template:

```powershell
scp -i .\sshkey.pem .\deploy\production\docker-daemon.json.example root@47.100.224.163:/etc/docker/daemon.json
ssh -i .\sshkey.pem root@47.100.224.163 "systemctl restart docker"
```

## Routine Update

Before a large version jump, copy the updated compose file and set `SETUP_MIGRATION_TIMEOUT_SECONDS` high enough for database migrations:

```powershell
scp -i .\sshkey.pem .\deploy\production\docker-compose.yml root@47.100.224.163:/opt/sub2api/docker-compose.yml
```

Pin `SUB2API_IMAGE` in `/opt/sub2api/.env` if needed, then deploy:

```bash
ssh -i .\sshkey.pem root@47.100.224.163 "bash /opt/sub2api/update.sh deploy"
```

Verify:

```bash
ssh -i .\sshkey.pem root@47.100.224.163 "bash /opt/sub2api/update.sh ps"
ssh -i .\sshkey.pem root@47.100.224.163 "curl -fsS http://127.0.0.1:8080/health"
ssh -i .\sshkey.pem root@47.100.224.163 "docker inspect sub2api --format 'image={{.Config.Image}} status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}no-health{{end}}'"
```

## Backup

Run before risky upgrades:

```bash
ssh -i .\sshkey.pem root@47.100.224.163 "bash /opt/sub2api/backup.sh"
```

Backups are written under `/opt/sub2api/backups`.

## Network Exposure

The container publishes `0.0.0.0:8080` by default. If `curl http://127.0.0.1:8080/health` works on the server but `http://47.100.224.163:8080/health` is unreachable externally, check the cloud security group first. On the server, UFW should not need changes unless it is explicitly enabled.
