# Research: Windows Claude / Obsidian diagnostic locations

Sources: official Claude Code docs (settings, env-vars, claude-directory), Obsidian Help data-storage, and a live probe on this Windows machine (2026-08-22).

## Claude Code

- Home config root: `%USERPROFILE%\.claude\` unless `CLAUDE_CONFIG_DIR` is set (then all `~/.claude` paths move there).
- Always also check `%USERPROFILE%\.claude.json` (session / MCP / global keys). Treat as secret-bearing.
- Useful files/dirs under the config root:
  - `settings.json`, `settings.local.json` — may contain `env.ANTHROPIC_BASE_URL` and keys
  - `.credentials.json` — exclude original; never copy raw
  - `debug/` — only when `--debug`, `/debug`, or debug env is on
  - `history.jsonl`, `transcripts/`, `projects/*.jsonl` — session text; default package must omit
  - `CLAUDE_CODE_DEBUG_LOGS_DIR` overrides the debug log **file** path (name is misleading)
- Node cache / MCP logs observed locally: `%LOCALAPPDATA%\claude-cli-nodejs\Cache\`
- Docs: https://code.claude.com/docs/en/settings , https://code.claude.com/docs/en/claude-directory , https://code.claude.com/docs/en/env-vars

## Claude Desktop

- Observed locally: `%LOCALAPPDATA%\Claude\Logs\` (`chrome-native-host.log`), `%LOCALAPPDATA%\Claude\claude_desktop_config.json`
- Also scan if present: `%APPDATA%\Claude\`, `%LOCALAPPDATA%\AnthropicClaude\`, `%APPDATA%\AnthropicClaude\`
- This machine had no `%APPDATA%\Claude`

## Obsidian

- Global settings / vault registry: `%APPDATA%\Obsidian\` (Help docs). Vault list file commonly `obsidian.json`.
- Each vault: `<vault>\.obsidian\plugins\<pluginId>\` — collect only plugin ids whose name contains `claude` or `anthropic` (case-insensitive), plus known aliases (`obsidian-copilot` only if its `data.json` mentions anthropic/claude).
- Never pack vault markdown / attachments.
- Manual extra vault: only `<chosen>\.obsidian\...` as above.

## Collector implications

- Read user + machine environment (process + HKCU/HKLM `Environment`) for `CLAUDE_CONFIG_DIR`, `ANTHROPIC_BASE_URL`, `ANTHROPIC_API_KEY` presence.
- Redact values; keep URL hostnames in the manifest.
- Missing roots are normal; mark not-found, still zip.
