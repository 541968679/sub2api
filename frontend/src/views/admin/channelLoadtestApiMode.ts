import type { LoadtestAPIMode } from '@/api/admin/channelLoadtest'

/**
 * Models that only accept native Chat Completions upstream.
 * Mirrors backend/internal/pkg/loadtest.RequiresNativeChatCompletions.
 * Extend this list when more vendors reject /v1/responses.
 */
const CHAT_COMPLETIONS_EXACT = new Set(['kimi'])
const CHAT_COMPLETIONS_PREFIXES = ['kimi-']

export function bareModelName(model: string): string {
  const trimmed = model.trim().toLowerCase()
  if (!trimmed) return ''
  const slash = trimmed.lastIndexOf('/')
  return slash >= 0 ? trimmed.slice(slash + 1) : trimmed
}

export function requiresChatCompletions(model: string): boolean {
  const bare = bareModelName(model)
  if (!bare) return false
  if (CHAT_COMPLETIONS_EXACT.has(bare)) return true
  return CHAT_COMPLETIONS_PREFIXES.some((prefix) => bare.startsWith(prefix))
}

export function parseModelList(raw: string): string[] {
  return raw
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

/** Default Responses; only when every listed model is a known CC-only model. */
export function suggestApiMode(modelsRaw: string): LoadtestAPIMode {
  const models = parseModelList(modelsRaw)
  if (models.length === 0) return 'responses'
  if (models.every(requiresChatCompletions)) return 'chat_completions'
  return 'responses'
}
