# Isolation 0.2.4 A-tier overlay status

Worktree: `E:\cursor project\api2sub-main-1`  
Branch: `sync/main-1`  
Freeze: `upstream/main` `98d86915b` / 0.2.4  
VERSION: **0.1.287** (not 0.2.4)  
Not merged to real `main`, not pushed, not deployed.

## Done

| Pack | Commit | Product |
|------|--------|---------|
| 0 leftovers | `f2da22463` | synthesized `created_at`, thinking bridges, cancel-before-close, fingerprint rewrite default-off, Ops return-to-list, usage request id, lite account list, Astra |
| 1 CN platforms | `76d0376b7` | Kimi / Zhipu / MiniMax / DeepSeek first-class; sessionToken kept |
| 2 Grok 4.6 | `bbabdb2f1` | catalog, xhigh, Chat→Responses media, same-account capacity 429; no official Grok price cards |
| 3 Model plaza | `03b0871f9` | `/model-plaza` + zh/en; paid prices = fork display chain; anonymous sees only non-exclusive groups |
| 4 Channel-monitor v2 | `1bbf7827d` | v2 API + admin UI; quota mode SQL 236; image-channel-monitor kept; default mode v1 |

## Residual (do not re-litigate)

- WS passthrough later-turn pre-output 429 reconnect (needs freeze WS v2 stack)
- Ollama Cloud usage-window hang-under-CN
- Grok Imagine media-eligibility admin UI on Edit Account
- Plaza freeze time-of-day / long-context stored-tier columns (billing lock)
- Billing lock: Fast stored billing, group force/free Fast, channel time prices, GPT Image 2.5 catalog/LiteLLM rates
- Passkey never; Go 1.27 / GHCR path swap never
- Channel-monitor live quota-fetch dispatch (CN usage-window fetcher) not wired
- Channel-monitor v2 popular-model seed not adopted

## Next pack

**5 — OAuth outbound plugins + claude-fable-5.1 + simple-mode grouping.** Plugins default off, unsigned rejected, API-key path not hijacked. Keep claude-fable-5; add 5.1.

Then 6 group allowlist default OFF, 7 Agent Identity, then waterline + this file’s final done/residual.
