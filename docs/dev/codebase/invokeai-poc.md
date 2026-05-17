# InvokeAI Canvas PoC

## Data model

This PoC does not add Sub2API database tables, migrations, services, or frontend
routes. InvokeAI keeps its own SQLite database, generated images, model cache,
and runtime configuration under an external root directory.

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
   API key as `external_openai_api_key`.
5. Sub2API routes/bills the request through its existing gateway/account stack.

## Important mechanisms

- InvokeAI is intentionally outside this repository and should not be imported
  or vendored into Sub2API.
- Runtime data is also outside the Sub2API repository so generated images,
  SQLite files, model caches, and node packs do not pollute this checkout.
- The local config currently contains a placeholder API key:
  `CHANGE_ME_SUB2API_KEY`. Replace it with a real Sub2API API key before testing
  image generation.
- For a quick local run from the source checkout:

```powershell
cd "E:\cursor project\InvokeAI"
.\.venv\Scripts\invokeai-web.exe --root "E:\cursor project\invokeai-sub2api-poc"
```

## Known pitfalls

- The InvokeAI source/dev install needs the React UI built into
  `invokeai/frontend/web/dist`; otherwise the backend starts without a UI.
- InvokeAI may not list a new upstream model name until the model entry is
  installed/configured in its External Providers UI. If `gpt-image-2` is not
  selectable, first validate with a listed GPT Image model or add an alias on
  the Sub2API side for the PoC.
- Do not use Sub2API's forbidden local ports. InvokeAI uses `9090` for this PoC,
  leaving Sub2API backend/frontend on `18081` and `15174`.
