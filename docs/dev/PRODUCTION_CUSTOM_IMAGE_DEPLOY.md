# Production Custom Image Deploy Guard

The known production main-app deployment builds Sub2API on the host from
`/opt/sub2api/repo` and tags the result as:

```text
sub2api-custom:latest
```

`deploy/update.sh` must keep Docker Compose pinned to that locally built image.
It does this by writing a managed file at:

```text
/opt/sub2api/docker-compose.override.yml
```

The managed override must resolve the `sub2api` service image to:

```text
sub2api-custom:latest
```

Do not rely on the base `docker-compose.yml` default image for this production
path. The base compose file intentionally keeps a public default image for
standalone installs, but this production host deploys custom code and therefore
must run the locally built image.

After every main-app deploy, verify both health and the exact running image:

```powershell
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "cd /opt/sub2api && docker compose ps sub2api"
ssh -i $HOME\.ssh\id_ed25519_sub2api root@172.245.247.80 "docker inspect sub2api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}}'"
```

Expected image name:

```text
sub2api-custom:latest
```

If the running image name is `weishaw/sub2api:latest`, Compose has restarted the
upstream image and the custom production code is not deployed. Treat that as a
failed deploy even if `/health` returns OK.
