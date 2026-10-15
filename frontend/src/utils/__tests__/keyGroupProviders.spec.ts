import { describe, expect, it } from 'vitest'
import { getKeyGroupProvider, keyGroupProviderForGroup } from '../keyGroupProviders'

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

  it('files an OpenAI-platform domestic group under 国产模型', () => {
    expect(keyGroupProviderForGroup({ platform: 'openai', name: '国产模型：聚合渠道' })).toBe('domestic')
    expect(keyGroupProviderForGroup({ platform: 'openai', name: 'gpt 每月2000刀' })).toBe('openai')
    expect(keyGroupProviderForGroup({ platform: 'kimi', name: '线路' })).toBe('domestic')
  })
})
