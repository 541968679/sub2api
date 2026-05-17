# InvokeAI Canvas PoC

## Data model

This PoC does not add Sub2API database tables, migrations, services, or frontend
routes. InvokeAI keeps its own SQLite database, generated images, model cache,
and runtime configuration under an external root directory.

The external InvokeAI checkout now stores OpenAI-compatible external provider
credentials per InvokeAI user in its own SQLite table,
`user_external_provider_configs`. API keys are stored in plaintext for the PoC;
only configured status and base URL are returned to the frontend.

Local paths:

- InvokeAI source checkout: `E:\cursor project\InvokeAI`
- InvokeAI runtime root: `E:\cursor project\invokeai-sub2api-poc`
- InvokeAI config: `E:\cursor project\invokeai-sub2api-poc\invokeai.yaml`

## Key files

- `E:\cursor project\InvokeAI\README.md` - upstream overview.
- `E:\cursor project\InvokeAI\docs\src\content\docs\start-here\manual.mdx` -
  upstream manual install instructions.
- `E:\cursor project\InvokeAI\docs\src\content\docs\configuration\invokeai-yaml.mdx` -
  upstream runtime config documentation.
- `E:\cursor project\InvokeAI\docs\src\content\docs\features\External Models\openai.mdx` -
  upstream OpenAI-compatible external image model setup.

## Core flow

1. Sub2API continues to run as the OpenAI-compatible API gateway on
   `http://127.0.0.1:18081`.
2. InvokeAI runs as an independent canvas/image UI on `http://127.0.0.1:9090`.
3. InvokeAI external OpenAI settings point at Sub2API:
   `external_openai_base_url: http://127.0.0.1:18081`.
4. InvokeAI sends image generation/edit requests to Sub2API with a Sub2API user
   API key saved on the current InvokeAI user.
5. External OpenAI image generation reads `queue_item.user_id`, fetches that
   user's provider config, and sends the request using that key/base URL.
6. Sub2API routes/bills the request through its existing gateway/account stack.

## Important mechanisms

- InvokeAI is intentionally outside this repository and should not be imported
  or vendored into Sub2API.
- Runtime data is also outside the Sub2API repository so generated images,
  SQLite files, model caches, and node packs do not pollute this checkout.
- The local config enables InvokeAI native multiuser mode with
  `multiuser: true` and `strict_password_checking: true`.
- The local PoC uses a built-in administrator instead of exposing a first-run
  setup flow. Local credentials are `admin` / `admin123`. For cloud deployment,
  change `builtin_admin_password` to a strong password or disable the local
  built-in-admin config and provision an admin out-of-band.
- Normal local start/restart/stop for InvokeAI should use the local script in
  the InvokeAI checkout. It fixes `host: 127.0.0.1`, `port: 9090`, multiuser
  settings, built-in admin settings, UTF-8 config encoding, process tracking,
  and log paths:

```powershell
cd "E:\cursor project\InvokeAI"
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 restart
```

or:

```bat
scripts\dev-stack.cmd restart
```

- In multiuser mode, the External Providers UI writes the current user's
  provider config instead of `api_keys.yaml`. Single-user mode keeps the old
  YAML-backed path.

## Known pitfalls

- The InvokeAI source/dev install needs the React UI built into
  `invokeai/frontend/web/dist`; otherwise the backend starts without a UI.
- InvokeAI may not list a new upstream model name until the model entry is
  installed/configured in its External Providers UI. If `gpt-image-2` is not
  selectable, first validate with a listed GPT Image model or add an alias on
  the Sub2API side for the PoC.
- Do not use Sub2API's forbidden local ports. InvokeAI uses `9090` for this PoC,
  leaving Sub2API backend/frontend on `18081` and `15174`.
- External starter model records remain instance-level. Deleting one user's
  provider config does not remove external model records, because other users
  may still rely on them.
