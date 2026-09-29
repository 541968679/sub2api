import { describe, expect, it } from 'vitest'
import {
  DEFAULT_OPENAI_RESPONSES_MODE,
  OPENAI_RESPONSES_MODE_AUTO,
  OPENAI_RESPONSES_MODE_FORCE_CHAT_COMPLETIONS,
  OPENAI_RESPONSES_MODE_FORCE_RESPONSES,
  OPENAI_RESPONSES_MODE_PASSTHROUGH,
  normalizeOpenAIResponsesMode,
  persistOpenAIResponsesMode
} from '@/utils/openaiResponsesMode'

describe('openaiResponsesMode utils', () => {
  it('defaults missing and invalid values to passthrough', () => {
    expect(DEFAULT_OPENAI_RESPONSES_MODE).toBe(OPENAI_RESPONSES_MODE_PASSTHROUGH)
    expect(normalizeOpenAIResponsesMode(undefined)).toBe(OPENAI_RESPONSES_MODE_PASSTHROUGH)
    expect(normalizeOpenAIResponsesMode('')).toBe(OPENAI_RESPONSES_MODE_PASSTHROUGH)
    expect(normalizeOpenAIResponsesMode('native')).toBe(OPENAI_RESPONSES_MODE_PASSTHROUGH)
    expect(normalizeOpenAIResponsesMode('enabled')).toBe(OPENAI_RESPONSES_MODE_PASSTHROUGH)
  })

  it('keeps explicit known modes', () => {
    expect(normalizeOpenAIResponsesMode('auto')).toBe(OPENAI_RESPONSES_MODE_AUTO)
    expect(normalizeOpenAIResponsesMode(' PASSTHROUGH ')).toBe(OPENAI_RESPONSES_MODE_PASSTHROUGH)
    expect(normalizeOpenAIResponsesMode('force_responses')).toBe(OPENAI_RESPONSES_MODE_FORCE_RESPONSES)
    expect(normalizeOpenAIResponsesMode('force_chat_completions')).toBe(
      OPENAI_RESPONSES_MODE_FORCE_CHAT_COMPLETIONS
    )
  })

  it('always persists the selected mode including auto', () => {
    const extra: Record<string, unknown> = {}
    persistOpenAIResponsesMode(extra, 'auto')
    expect(extra.openai_responses_mode).toBe(OPENAI_RESPONSES_MODE_AUTO)
    persistOpenAIResponsesMode(extra, undefined)
    expect(extra.openai_responses_mode).toBe(OPENAI_RESPONSES_MODE_PASSTHROUGH)
  })
})
