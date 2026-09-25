import { describe, expect, it } from 'vitest'

import { requiresChatCompletions, suggestApiMode } from '@/views/admin/channelLoadtestApiMode'

describe('channelLoadtestApiMode', () => {
  it('marks kimi models as chat-completions-only', () => {
    expect(requiresChatCompletions('kimi-k3')).toBe(true)
    expect(requiresChatCompletions('vendor/kimi-k2')).toBe(true)
    expect(requiresChatCompletions('glm-5.3')).toBe(false)
    expect(requiresChatCompletions('gpt-5.4')).toBe(false)
  })

  it('suggests responses by default and cc when every model is kimi', () => {
    expect(suggestApiMode('')).toBe('responses')
    expect(suggestApiMode('glm-5.3')).toBe('responses')
    expect(suggestApiMode('gpt-5.4, glm-5.3')).toBe('responses')
    expect(suggestApiMode('kimi-k3')).toBe('chat_completions')
    expect(suggestApiMode('kimi-k3, kimi-k2')).toBe('chat_completions')
    // Mixed list keeps Responses; engine still forces kimi rows to CC per request.
    expect(suggestApiMode('kimi-k3, glm-5.3')).toBe('responses')
  })
})
