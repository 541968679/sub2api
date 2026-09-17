import { describe, expect, it } from 'vitest'
import { inferModelPricingProvider } from '../modelPricingOptions'

describe('inferModelPricingProvider', () => {
  it('maps domestic coding IDs to openai so 模型配置 stays on the OpenAI tab', () => {
    expect(inferModelPricingProvider('glm-5.3')).toBe('openai')
    expect(inferModelPricingProvider('kimi-k2.5')).toBe('openai')
    expect(inferModelPricingProvider('deepseek-v4-flash')).toBe('openai')
    expect(inferModelPricingProvider('MiniMax-M2.5')).toBe('openai')
    expect(inferModelPricingProvider('moonshot-v1-128k')).toBe('openai')
  })

  it('keeps existing brand prefixes', () => {
    expect(inferModelPricingProvider('claude-opus-4-6')).toBe('anthropic')
    expect(inferModelPricingProvider('gemini-2.5-pro')).toBe('gemini')
    expect(inferModelPricingProvider('gpt-5.6-sol')).toBe('openai')
  })
})
