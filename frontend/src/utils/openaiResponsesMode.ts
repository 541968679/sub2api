export const OPENAI_RESPONSES_MODE_AUTO = 'auto'
export const OPENAI_RESPONSES_MODE_FORCE_RESPONSES = 'force_responses'
export const OPENAI_RESPONSES_MODE_FORCE_CHAT_COMPLETIONS = 'force_chat_completions'
export const OPENAI_RESPONSES_MODE_PASSTHROUGH = 'passthrough'

export type OpenAIResponsesMode =
  | typeof OPENAI_RESPONSES_MODE_AUTO
  | typeof OPENAI_RESPONSES_MODE_FORCE_RESPONSES
  | typeof OPENAI_RESPONSES_MODE_FORCE_CHAT_COMPLETIONS
  | typeof OPENAI_RESPONSES_MODE_PASSTHROUGH

export const DEFAULT_OPENAI_RESPONSES_MODE: OpenAIResponsesMode =
  OPENAI_RESPONSES_MODE_PASSTHROUGH

const OPENAI_RESPONSES_MODES = new Set<OpenAIResponsesMode>([
  OPENAI_RESPONSES_MODE_AUTO,
  OPENAI_RESPONSES_MODE_FORCE_RESPONSES,
  OPENAI_RESPONSES_MODE_FORCE_CHAT_COMPLETIONS,
  OPENAI_RESPONSES_MODE_PASSTHROUGH
])

export const normalizeOpenAIResponsesMode = (mode: unknown): OpenAIResponsesMode => {
  if (typeof mode !== 'string') return DEFAULT_OPENAI_RESPONSES_MODE
  const normalized = mode.trim().toLowerCase()
  if (OPENAI_RESPONSES_MODES.has(normalized as OpenAIResponsesMode)) {
    return normalized as OpenAIResponsesMode
  }
  return DEFAULT_OPENAI_RESPONSES_MODE
}

export const persistOpenAIResponsesMode = (
  extra: Record<string, unknown>,
  mode: unknown
): void => {
  extra.openai_responses_mode = normalizeOpenAIResponsesMode(mode)
}
