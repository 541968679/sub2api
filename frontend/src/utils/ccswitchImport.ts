import type { GroupPlatform } from '@/types'

/**
 * Default Codex model written into CC Switch for Grok-group keys.
 * Matches UseKeyModal / Grok CLI defaults (OpenAI-compatible Responses).
 */
export const GROK_CC_SWITCH_CODEX_MODEL = 'grok-4.5'

/** Admin group-form preset: latest OpenAI display-catalog model. */
export const CCS_IMPORT_PRESET_GPT = 'gpt-6-astra'

/** Admin group-form preset: GLM 5.3 for domestic OpenAI groups. */
export const CCS_IMPORT_PRESET_GLM = 'glm-5.3'

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

export function shouldShowCcsCodexModelPicker(
  pickerEnabled: boolean | undefined,
  clientType: CcSwitchClientType
): boolean {
  return clientType === 'codex' && pickerEnabled === true
}

/** True when a /v1/models payload is the stock OpenAI/Grok catalog, not domestic IDs. */
export function looksLikeOpenAIDisplayCatalog(ids: string[]): boolean {
  if (ids.length === 0) return true
  return ids.every((id) => {
    const model = id.trim().toLowerCase()
    if (!model) return true
    return (
      model.startsWith('gpt-') ||
      model.startsWith('grok-') ||
      model.startsWith('chatgpt') ||
      model.startsWith('o1') ||
      model.startsWith('o3') ||
      model.startsWith('o4')
    )
  })
}

/** Current-generation CCS default candidates. Drops glm-4.x / distill / snapshots. */
export function isCurrentCcsImportModel(id: string): boolean {
  const model = id.trim().toLowerCase()
  if (!model) return false
  return (
    model.startsWith('gpt-6') ||
    model.startsWith('glm-5.3') ||
    model === 'kimi-k3' ||
    model.startsWith('kimi-k3-') ||
    model.startsWith('kimi-k2.5') ||
    model.startsWith('kimi-k2.6') ||
    model === 'kimi-k2-thinking' ||
    model.startsWith('deepseek-v4') ||
    model === 'minimax-m2' ||
    model.startsWith('minimax-m2.') ||
    model.startsWith('minimax-m3') ||
    model.startsWith('grok-4.5') ||
    model.startsWith('grok-4.6')
  )
}

export function compactCcsImportModelIDs(ids: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const id of ids) {
    const trimmed = id.trim()
    if (!trimmed || !isCurrentCcsImportModel(trimmed)) continue
    const key = trimmed.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(trimmed)
  }
  return out
}

export function ccsImportFamilyFallbackIDs(defaultModel: string): string[] {
  const model = defaultModel.trim().toLowerCase()
  if (model.startsWith('glm-') || model.includes('chatglm')) {
    return ['glm-5.3', 'glm-5.3-flash']
  }
  if (model.startsWith('kimi-') || model.startsWith('moonshot-')) {
    return ['kimi-k2.5', 'kimi-k2.6', 'kimi-k3', 'kimi-k2-thinking']
  }
  if (model.startsWith('deepseek-')) {
    return ['deepseek-v4-pro', 'deepseek-v4-flash']
  }
  if (model.startsWith('minimax-')) {
    return ['MiniMax-M2.5', 'MiniMax-M2.1']
  }
  if (
    model.startsWith('gpt-') ||
    model.startsWith('chatgpt') ||
    model.startsWith('o1') ||
    model.startsWith('o3') ||
    model.startsWith('o4')
  ) {
    return ['gpt-6-astra']
  }
  if (model.startsWith('grok-')) {
    return ['grok-4.5', 'grok-4.6']
  }
  return []
}

function uniqueTrimmedModelIDs(ids: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const id of ids) {
    const trimmed = id.trim()
    if (!trimmed) continue
    const key = trimmed.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(trimmed)
  }
  return out
}

/**
 * Build the CCS Codex import picker list.
 * Account mapping keys win (unfiltered). A stock GPT /v1/models catalog is
 * ignored so it does not replace the group whitelist. Family fallback is only
 * used when both sources are empty.
 */
export function resolveCcsImportPickerIDs(options: {
  defaultModel: string
  fetchedIDs?: string[]
  accountIDs?: string[]
}): string[] {
  const accountIDs = uniqueTrimmedModelIDs(options.accountIDs ?? [])
  const fetched = options.fetchedIDs ?? []
  const fetchedIDs = looksLikeOpenAIDisplayCatalog(fetched) ? [] : uniqueTrimmedModelIDs(fetched)

  let candidates = mergeCcsImportModelOptions('', [...accountIDs, ...fetchedIDs])
  if (candidates.length === 0) {
    candidates = ccsImportFamilyFallbackIDs(options.defaultModel)
  }
  return mergeCcsImportModelOptions(options.defaultModel, candidates)
}

export function filterCcsImportModelIDs(ids: string[], query: string): string[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return ids
  return ids.filter((id) => id.toLowerCase().includes(needle))
}

/** @deprecated use resolveCcsImportPickerIDs */
export function resolveCcsImportDropdownIDs(options: {
  defaultModel: string
  fetchedIDs: string[]
  fallbackIDs?: string[]
}): string[] {
  return resolveCcsImportPickerIDs({
    defaultModel: options.defaultModel,
    fetchedIDs: options.fetchedIDs,
    accountIDs: looksLikeOpenAIDisplayCatalog(options.fetchedIDs) ? options.fallbackIDs : undefined
  })
}

export function mergeCcsImportModelOptions(defaultModel: string, ids: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  const trimmedDefault = defaultModel.trim()
  if (trimmedDefault) {
    out.push(trimmedDefault)
    seen.add(trimmedDefault)
  }
  for (const id of ids) {
    const trimmed = id.trim()
    if (!trimmed || seen.has(trimmed)) continue
    seen.add(trimmed)
    out.push(trimmed)
  }
  return out
}

/** Parse OpenAI/Claude-style `{ object, data: [{ id }] }` from GET /v1/models. */
export function parseGatewayModelsList(payload: unknown): string[] {
  if (!payload || typeof payload !== 'object') return []
  const data = (payload as { data?: unknown }).data
  if (!Array.isArray(data)) return []
  const ids: string[] = []
  for (const item of data) {
    if (!item || typeof item !== 'object') continue
    const id = (item as { id?: unknown }).id
    if (typeof id !== 'string') continue
    const trimmed = id.trim()
    if (trimmed) ids.push(trimmed)
  }
  return ids
}

/**
 * Open a `ccswitch://` deeplink from a user gesture.
 *
 * Browsers cannot reliably report whether a custom protocol handler succeeded.
 * Focus/visibility heuristics (e.g. document.hasFocus after a short timeout)
 * produce false "not installed" errors on Windows when CC-Switch opens without
 * stealing browser focus. Callers should treat launch as best-effort and show
 * an optimistic success message with a soft fallback, not a hard failure.
 */
export function launchCcSwitchImportDeeplink(deeplink: string): void {
  if (typeof document === 'undefined') {
    throw new Error('ccswitch deeplink launch requires a browser document')
  }

  // Prefer a same-document <a> click: stays in the user-gesture chain and does
  // not depend on popup/window.open behavior for custom schemes.
  const anchor = document.createElement('a')
  anchor.href = deeplink
  anchor.rel = 'noopener noreferrer'
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  try {
    anchor.click()
  } finally {
    anchor.remove()
  }
}
