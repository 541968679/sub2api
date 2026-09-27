import { describe, expect, it } from 'vitest'
import { getKeyGroupProvider } from '../keyGroupProviders'

describe('getKeyGroupProvider', () => {
  it('keeps Claude and OpenAI on their own buttons', () => {
    expect(getKeyGroupProvider('anthropic')).toBe('anthropic')
    expect(getKeyGroupProvider('openai')).toBe('openai')
  })

  it('groups domestic coding platforms together', () => {
    expect(getKeyGroupProvider('deepseek')).toBe('domestic')
    expect(getKeyGroupProvider('kimi')).toBe('domestic')
    expect(getKeyGroupProvider('zhipu')).toBe('domestic')
    expect(getKeyGroupProvider('minimax')).toBe('domestic')
  })

  it('puts Gemini, Grok, and Antigravity under other', () => {
    expect(getKeyGroupProvider('gemini')).toBe('other')
    expect(getKeyGroupProvider('grok')).toBe('other')
    expect(getKeyGroupProvider('antigravity')).toBe('other')
  })
})
