import { describe, expect, it } from 'vitest'
import {
  GROK_CODEX_CATALOG_FILENAME,
  GROK_CODEX_DEFAULT_MODEL,
  buildGrokCodexConfigToml,
  buildGrokCodexModelCatalogJson
} from '@/utils/codexGrokCatalog'

describe('codexGrokCatalog', () => {
  it('builds a catalog entry for grok-4.5 with Codex-required ModelInfo fields', () => {
    const raw = buildGrokCodexModelCatalogJson()
    const parsed = JSON.parse(raw)
    expect(parsed.models).toHaveLength(1)
    const model = parsed.models[0]
    expect(model.slug).toBe(GROK_CODEX_DEFAULT_MODEL)
    expect(model.context_window).toBe(1_000_000)
    expect(String(model.base_instructions).length).toBeGreaterThan(20)
    // Codex serde fails hard if these are omitted (startup: missing field ...).
    expect(model.supports_reasoning_summaries).toBe(true)
    expect(model.supports_parallel_tool_calls).toBe(true)
    expect(model.apply_patch_tool_type).toBe('freeform')
    expect(model.tool_mode).toBe('code_mode_only')
    expect(model.default_reasoning_summary).toBe('none')
    expect(model.support_verbosity).toBe(true)
    expect(model.supports_search_tool).toBe(true)
  })

  it('writes Codex config with context window and relative catalog path', () => {
    const toml = buildGrokCodexConfigToml({
      baseUrl: 'https://example.com/v1',
      providerName: 'ZeroCode'
    })
    expect(toml).toContain('model = "grok-4.5"')
    expect(toml).toContain('model_context_window = 1000000')
    expect(toml).toContain(`model_catalog_json = "${GROK_CODEX_CATALOG_FILENAME}"`)
    expect(toml).toContain('base_url = "https://example.com/v1"')
    expect(toml).toContain('wire_api = "responses"')
    expect(toml).not.toContain('claude-sonnet')
  })

  it('optionally enables Responses WebSocket v2 for Codex', () => {
    const toml = buildGrokCodexConfigToml({
      baseUrl: 'https://example.com/v1',
      supportsWebsockets: true
    })
    expect(toml).toContain('supports_websockets = true')
    expect(toml).toContain('responses_websockets_v2 = true')
  })
})
