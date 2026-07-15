/** Default Grok model id used for Codex CLI / CCS-style Responses imports. */
export const GROK_CODEX_DEFAULT_MODEL = 'grok-4.5'

/** Relative path Codex resolves from ~/.codex (portable across OS / WSL). */
export const GROK_CODEX_CATALOG_FILENAME = 'model-catalog-grok.json'

const GROK_BASE_INSTRUCTIONS =
  'You are a coding agent powered by Grok. Collaborate with the user until their goal is handled. Prefer clear, correct, minimal diffs. Follow repository conventions and existing patterns. When uncertain, inspect the code before changing it.'

/**
 * Codex ModelInfo catalog for `grok-4.5`.
 *
 * Fields mirror Codex CLI's strict ModelInfo schema (serde): omitting required
 * booleans such as `supports_reasoning_summaries` fails startup with
 * "failed to parse model_catalog_json".
 */
export function buildGrokCodexModelCatalogJson(
  model: string = GROK_CODEX_DEFAULT_MODEL
): string {
  const catalog = {
    models: [
      {
        slug: model,
        display_name: 'Grok 4.5',
        description: 'xAI Grok 4.5 via Sub2API (OpenAI-compatible Responses).',
        // xAI Grok accepts low/medium/high only (no xhigh/extra-high).
        default_reasoning_level: 'high',
        supported_reasoning_levels: [
          { effort: 'low', description: 'Faster responses' },
          { effort: 'medium', description: 'Balanced' },
          { effort: 'high', description: 'Deeper reasoning' }
        ],
        shell_type: 'shell_command',
        visibility: 'list',
        supported_in_api: true,
        priority: 0,
        base_instructions: GROK_BASE_INSTRUCTIONS,
        model_messages: {
          instructions_template: GROK_BASE_INSTRUCTIONS,
          instructions_variables: {},
          approvals: null
        },
        context_window: 1_000_000,
        max_context_window: 1_000_000,
        effective_context_window_percent: 90,
        input_modalities: ['text', 'image'],
        supports_parallel_tool_calls: true,
        supports_reasoning_summaries: true,
        default_reasoning_summary: 'none',
        support_verbosity: true,
        default_verbosity: 'low',
        apply_patch_tool_type: 'freeform',
        tool_mode: 'code_mode_only',
        truncation_policy: { mode: 'tokens', limit: 10000 },
        supports_search_tool: true,
        web_search_tool_type: 'text_and_image',
        supports_image_detail_original: true,
        use_responses_lite: true,
        include_skills_usage_instructions: false,
        experimental_supported_tools: [] as string[],
        upgrade: null,
        multi_agent_version: 'v2'
      }
    ]
  }
  return `${JSON.stringify(catalog, null, 2)}\n`
}

export interface GrokCodexConfigOptions {
  baseUrl: string
  model?: string
  supportsWebsockets?: boolean
  providerName?: string
}

/**
 * Codex `config.toml` for a Grok-group Sub2API key.
 * Includes context window + catalog pointer so Codex does not use fallback metadata.
 */
export function buildGrokCodexConfigToml(options: GrokCodexConfigOptions): string {
  const model = options.model?.trim() || GROK_CODEX_DEFAULT_MODEL
  const providerName = options.providerName?.trim() || 'Sub2API'
  const baseUrl = options.baseUrl.replace(/\/+$/, '')
  const wsLines = options.supportsWebsockets
    ? `\nsupports_websockets = true\n\n[features]\nresponses_websockets_v2 = true\n`
    : '\n'

  return `model_provider = "custom"
model = "${model}"
model_reasoning_effort = "high"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true
model_context_window = 1000000
model_auto_compact_token_limit = 900000
model_catalog_json = "${GROK_CODEX_CATALOG_FILENAME}"

[model_providers.custom]
name = "${providerName}"
base_url = "${baseUrl}"
wire_api = "responses"
requires_openai_auth = true${wsLines}`
}
