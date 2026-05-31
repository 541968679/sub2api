# InvokeAI Sidecar Deployment

## Purpose

This project uses InvokeAI as an external API image-generation UI/API sidecar for Sub2API. It follows the same child-project pattern as AIClient2API:

```text
git push InvokeAI fork
  -> GitHub Actions buildx
  -> ghcr.io/541968679/invokeai-sub2api:latest
  -> /opt/sub2api/update.sh --only-invokeai
  -> docker compose pull invokeai && docker compose up -d invokeai
```

InvokeAI source code stays in its own GitHub repository. The Sub2API repository only owns production orchestration: Compose service, update script, and operations documentation.

## Repositories And Image

| Item | Value |
|---|---|
| Upstream | `invoke-ai/InvokeAI` |
| Fork | `541968679/invokeai-sub2api` |
| Local path | `E:\cursor project\InvokeAI` |
| Production image | `ghcr.io/541968679/invokeai-sub2api:latest` |
| Sub2API sidecar service | `invokeai` |

The InvokeAI GitHub Actions workflow builds `docker/Dockerfile` with:

```text
GPU_DRIVER=cpu
platforms=linux/amd64
```

This is intentional. This deployment is API-client-only and must not depend on CUDA, GPUs, local model files, or local image inference.

## Production Runtime

| Item | Value |
|---|---|
| Compose directory | `/opt/sub2api` |
| InvokeAI root | `/opt/invokeai/root` |
| Container root | `/invokeai` |
| Host bind | `127.0.0.1:9090` |
| Docker DNS | `http://invokeai:9090` |
| Public debug URL | `https://invokeai.172.245.247.80.sslip.io` |
| Health endpoint | `http://127.0.0.1:9090/api/v1/auth/status` |

Required production `.env` entries:

```dotenv
INVOKEAI_IMAGE=ghcr.io/541968679/invokeai-sub2api:latest
INVOKEAI_PORT=9090
INVOKEAI_MULTIUSER=true
INVOKEAI_STRICT_PASSWORD_CHECKING=true
INVOKEAI_BUILTIN_ADMIN_ENABLED=true
INVOKEAI_BUILTIN_ADMIN_USERNAME=admin
INVOKEAI_BUILTIN_ADMIN_PASSWORD=<strong-production-password>
INVOKEAI_SESSION_QUEUE_CONCURRENCY=4
```

The Compose service also pins:

```dotenv
INVOKEAI_DEVICE=cpu
INVOKEAI_PRECISION=float32
```

Do not override these to GPU values in this deployment.

## Deployment Commands

```powershell
# Deploy only InvokeAI
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh --only-invokeai"

# Full deploy: Sub2API + AIClient2API + InvokeAI
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh"

# Inspect runtime state
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose ps invokeai"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose logs --tail=120 invokeai"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "curl -fsS http://127.0.0.1:9090/api/v1/auth/status"
```

If the GHCR package is private, log in once on the production server:

```bash
docker login ghcr.io
```

## First Deployment Checklist

1. Create or confirm the GitHub fork `541968679/invokeai-sub2api`.
2. Push the local InvokeAI fork to `origin/main`.
3. Confirm GitHub Actions published `ghcr.io/541968679/invokeai-sub2api:latest`.
4. On the server, set `INVOKEAI_BUILTIN_ADMIN_PASSWORD` in `/opt/sub2api/.env`.
5. Run `bash /opt/sub2api/update.sh --only-invokeai`.
6. Verify `docker compose ps invokeai` and the `/api/v1/auth/status` health endpoint.
7. Add a Caddy/Nginx vhost only if a public Web UI endpoint is needed; keep the Docker service loopback-bound.

## Public Debug Endpoint

Current production Caddy vhost:

```caddyfile
invokeai.172.245.247.80.sslip.io {
	encode zstd gzip

	request_body {
		max_size 256MB
	}

	reverse_proxy 127.0.0.1:9090 {
		flush_interval -1
	}
}
```

This is a DNS-free debug domain backed by `sslip.io`. Replace it with a managed
project domain, for example `invokeai.zerocode.kaynlab.com`, after the DNS A
record points to `172.245.247.80`.

## Security Notes

- Do not commit InvokeAI admin passwords, OpenAI/Sub2API API keys, or provider credentials.
- Keep multiuser mode enabled in production.
- User creation and provider settings should be managed through the authenticated InvokeAI admin UI/API.
- Do not mount local model directories or GPU devices into the container.
