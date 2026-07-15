import type { GroupPlatform } from '@/types'

/**
 * Default Codex model written into CC Switch for Grok-group keys.
 * Matches UseKeyModal / Grok CLI defaults (OpenAI-compatible Responses).
 */
export const GROK_CC_SWITCH_CODEX_MODEL = 'grok-4.5'

/** CC Switch deeplink app types offered by Sub2API. */
export type CcSwitchClientType = 'claude' | 'gemini' | 'codex'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportModelOptions {
  /** Admin public setting `ccs_import_codex_model` (OpenAI → Codex). */
  openaiCodexModel?: string
  /** Admin public setting `ccs_import_anthropic_codex_model` (Anthropic key → Codex). */
  anthropicCodexModel?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
  modelOptions?: CcSwitchImportModelOptions
}

/**
 * Resolve CC Switch import app/endpoint/model from group platform.
 * Structure aligns with upstream `ccswitchImport`; fork adds Codex client
 * selection for Anthropic and an explicit Grok branch (upstream defaulted
 * unknown platforms to Claude without a model, which is wrong for Grok).
 */
export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string,
  modelOptions: CcSwitchImportModelOptions = {}
): CcSwitchImportConfig {
  const openaiModel = modelOptions.openaiCodexModel?.trim() || ''
  const anthropicCodexModel = modelOptions.anthropicCodexModel?.trim() || ''

  switch (platform || 'anthropic') {
    case 'antigravity':
      if (clientType === 'gemini') {
        return { app: 'gemini', endpoint: `${baseUrl}/antigravity` }
      }
      if (clientType === 'codex') {
        return {
          app: 'codex',
          endpoint: `${baseUrl}/antigravity`,
          ...(anthropicCodexModel ? { model: anthropicCodexModel } : {})
        }
      }
      return { app: 'claude', endpoint: `${baseUrl}/antigravity` }

    case 'openai':
      return {
        app: 'codex',
        endpoint: baseUrl,
        ...(openaiModel ? { model: openaiModel } : {})
      }

    case 'grok':
      // Grok is OpenAI-compatible Responses; import as Codex with Grok model.
      return {
        app: 'codex',
        endpoint: baseUrl,
        model: GROK_CC_SWITCH_CODEX_MODEL
      }

    case 'gemini':
      return { app: 'gemini', endpoint: baseUrl }

    default:
      // anthropic (+ any unknown): honor selected client
      if (clientType === 'codex') {
        return {
          app: 'codex',
          endpoint: baseUrl,
          ...(anthropicCodexModel ? { model: anthropicCodexModel } : {})
        }
      }
      if (clientType === 'gemini') {
        return { app: 'gemini', endpoint: baseUrl }
      }
      return { app: 'claude', endpoint: baseUrl }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(
    input.platform,
    input.clientType,
    input.baseUrl,
    input.modelOptions
  )
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', input.baseUrl],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    // Keep model near app (same placement as upstream ccswitchImport).
    entries.splice(2, 0, ['model', config.model])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
