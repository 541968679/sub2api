import { describe, expect, it } from 'vitest'
import type { PricingPageModel, PricingPagePlatform } from '@/api/pricingPage'
import {
  DOMESTIC_PRICING_TAB,
  filterPricingModels,
  isDomesticPricingModel,
  splitDomesticPricingPlatforms
} from '../pricingPageModels'

function tokenModel(id: string, input = 0.000001): PricingPageModel {
  return {
    model: id,
    billing_mode: 'per_token',
    display_input_price: input,
    display_output_price: input * 2,
    display_cache_read_price: input / 5,
    per_request_price: null
  }
}

describe('isDomesticPricingModel', () => {
  it('classifies CN coding IDs and leaves GPT/Claude/Grok alone', () => {
    expect(isDomesticPricingModel('glm-5.3')).toBe(true)
    expect(isDomesticPricingModel('Kimi-K2.7-code')).toBe(true)
    expect(isDomesticPricingModel('deepseek-v4-flash')).toBe(true)
    expect(isDomesticPricingModel('MiniMax-M2.7')).toBe(true)
    expect(isDomesticPricingModel('qwen-3.8-max')).toBe(true)
    expect(isDomesticPricingModel('mimo-v2.5-pro')).toBe(true)
    expect(isDomesticPricingModel('Hy4-preview')).toBe(true)
    expect(isDomesticPricingModel('gpt-5.6-luna')).toBe(false)
    expect(isDomesticPricingModel('claude-sonnet-4')).toBe(false)
    expect(isDomesticPricingModel('grok-4.6')).toBe(false)
    expect(isDomesticPricingModel('o3-mini')).toBe(false)
  })
})

describe('splitDomesticPricingPlatforms', () => {
  it('moves domestic IDs out of openai into a sibling tab', () => {
    const platforms: PricingPagePlatform[] = [
      { provider: 'anthropic', models: [tokenModel('claude-sonnet-4')] },
      { provider: 'openai', models: [tokenModel('gpt-5'), tokenModel('glm-5.3'), tokenModel('kimi-k3')] }
    ]
    const split = splitDomesticPricingPlatforms(platforms)
    expect(split.map((p) => p.provider)).toEqual(['anthropic', 'openai', DOMESTIC_PRICING_TAB])
    expect(split[1].models.map((m) => m.model)).toEqual(['gpt-5'])
    expect(split[2].models.map((m) => m.model)).toEqual(['glm-5.3', 'kimi-k3'])
  })

  it('replaces an openai-only domestic list with the domestic tab', () => {
    const split = splitDomesticPricingPlatforms([
      { provider: 'openai', models: [tokenModel('glm-5.3-flash')] }
    ])
    expect(split).toEqual([
      { provider: DOMESTIC_PRICING_TAB, models: [tokenModel('glm-5.3-flash')] }
    ])
  })
})

describe('filterPricingModels', () => {
  const models: PricingPageModel[] = [
    {
      ...tokenModel('gpt-5', 0.000001),
      display_output_price: 0.000003
    },
    {
      ...tokenModel('glm-5.3', 0.000002),
      display_output_price: 0.00003
    }
  ]

  it('filters by model name tokens', () => {
    expect(filterPricingModels(models, 'glm 5.3').map((m) => m.model)).toEqual(['glm-5.3'])
  })

  it('filters by displayed MTok price without substring false positives', () => {
    expect(filterPricingModels(models, '2.00').map((m) => m.model)).toEqual(['glm-5.3'])
    expect(filterPricingModels(models, '¥2.00', 1).map((m) => m.model)).toEqual(['glm-5.3'])
    expect(filterPricingModels(models, '1.00').map((m) => m.model)).toEqual(['gpt-5'])
  })
})
