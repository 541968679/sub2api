# Deprecated: Production Custom Image Deploy Guard

This document records a retired production path. Do not use it for future
Sub2API main-service deploys.

As of 2026-06-01, the Sub2API main service must be deployed from a GitHub
Actions-built GHCR image:

```text
ghcr.io/541968679/sub2api:latest
```

An explicitly approved immutable tag is also acceptable. Production deploys
must pull the published image with Docker Compose and restart the service from
that image. Do not run `docker build` on the production host for the Sub2API
main service.

## Last Verified GHCR Deploy

The latest verified production main-service deployment was completed on
2026-06-03.

| Tag | Revision | Image | Version label | Status |
|-----|----------|-------|---------------|--------|
| `v0.1.137` | `e385b9ac7d7e840658cbcb4f7f9f8f11b1954b81` | `ghcr.io/541968679/sub2api:latest` | `0.1.137` | running, healthy, `/health` OK |

Pushing `main` alone does not publish a fresh GHCR `latest` image in the
current workflow. The release workflow publishes images only from a `v*` tag
push or `workflow_dispatch`, so verify the intended tag or `latest` manifest
before any production `docker compose pull/up`.

## Retired Path

The previous production main-app deployment built Sub2API on the host from
`/opt/sub2api/repo` and pinned Compose to:

```text
sub2api-custom:latest
```

That path is now legacy. Treat any future main-service deploy that starts
`sub2api-custom:*` as an incorrect deploy, even if `/health` returns OK.

## Required Next Main-Service Deploy Shape

Before the next Sub2API main-service deployment:

1. Confirm GitHub Actions has published `ghcr.io/541968679/sub2api:latest` or
   the explicitly approved tag for the commit being deployed.
2. Ensure production Compose resolves the `sub2api` service image to that GHCR
   image. Remove or replace the historical generated override at
   `/opt/sub2api/docker-compose.override.yml` if it still pins
   `sub2api-custom:latest`.
3. Deploy by pulling and restarting the GHCR image:

```powershell
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose pull sub2api && docker compose up -d --no-deps sub2api"
```

4. Verify both health and the exact running image:

```powershell
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose ps sub2api"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "docker inspect sub2api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}}'"
```

Expected image name:

```text
ghcr.io/541968679/sub2api:latest
```

or the explicitly approved GHCR tag for that deploy.
