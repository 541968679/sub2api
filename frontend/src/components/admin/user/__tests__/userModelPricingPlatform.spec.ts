import { describe, expect, it } from 'vitest'
import {
  applySuggestedBoth,
  applySuggestedToCurated,
  emptyCatalogMap,
  emptyOverrideRow,
  ensureCuratedOverrideRows,
  litellmSuggestedMTok,
  platformsForModel,
  rowVisibleOnTab,
  type PricingProviderHint,
  type SuggestedPriceSource,
} from '../userModelPricingPlatform'

function item(model: string, provider?: string): PricingProviderHint {
  return { model, provider: provider || '', global_override: null, billing_basis_hint: undefined }
}

describe('platformsForModel / rowVisibleOnTab', () => {
  const catalogs = {
    ...emptyCatalogMap(),
    anthropic: ['claude-opus-4-8', 'claude-sonnet-4-6'],
    antigravity: ['claude-opus-4-8', 'claude-haiku-4-5'],
    openai: ['gpt-5.5', 'grok-4.5'],
    gemini: ['gemini-2.5-pro'],
  }

  it('puts overlapping Claude ids on both Anthropic and Antigravity', () => {
    expect(platformsForModel('claude-opus-4-8', catalogs)).toEqual(['anthropic', 'antigravity'])
    expect(rowVisibleOnTab({ model: 'claude-opus-4-8' }, 'anthropic', catalogs)).toBe(true)
    expect(rowVisibleOnTab({ model: 'claude-opus-4-8' }, 'antigravity', catalogs)).toBe(true)
    expect(rowVisibleOnTab({ model: 'claude-opus-4-8' }, 'other', catalogs)).toBe(false)
  })

  it('keeps OpenAI catalog grok-4.5 on OpenAI even though name inference is empty', () => {
    expect(platformsForModel('grok-4.5', catalogs)).toEqual(['openai'])
    expect(rowVisibleOnTab({ model: 'grok-4.5' }, 'openai', catalogs)).toBe(true)
    expect(rowVisibleOnTab({ model: 'grok-4.5' }, 'other', catalogs)).toBe(false)
  })

  it('classifies uncatalogued Claude ids as Anthropic via name inference', () => {
    expect(platformsForModel('claude-3-opus', catalogs, item('claude-3-opus'))).toEqual(['anthropic'])
  })

  it('does not dump unknown models onto Antigravity via resolveModelPricingProvider default', () => {
    expect(platformsForModel('mystery-model', catalogs, item('mystery-model', 'unknown'))).toEqual([])
    expect(rowVisibleOnTab({ model: 'mystery-model' }, 'other', catalogs, item('mystery-model'))).toBe(true)
    expect(rowVisibleOnTab({ model: 'mystery-model' }, 'antigravity', catalogs, item('mystery-model'))).toBe(false)
  })

  it('keeps empty new rows on the tab they were added from', () => {
    expect(rowVisibleOnTab({ model: '', addedOnTab: 'openai' }, 'openai', catalogs)).toBe(true)
    expect(rowVisibleOnTab({ model: '', addedOnTab: 'openai' }, 'other', catalogs)).toBe(false)
  })
})

describe('apply suggested + curated ensure', () => {
  const source: SuggestedPriceSource = {
    input_price: 5e-6,
    output_price: 2.5e-5,
    cache_write_price: 6.25e-6,
    cache_write_1h_price: 1e-5,
    cache_read_price: 5e-7,
  }

  it('fills billing and display from the same LiteLLM suggested mapping as per-row buttons', () => {
    const row = emptyOverrideRow('claude-opus-4-8', 'anthropic')
    applySuggestedBoth(row, (field) => litellmSuggestedMTok(source, field))
    expect(row.input_price).toBe(5)
    expect(row.output_price).toBe(25)
    expect(row.cache_write_price).toBe(6.25)
    expect(row.cache_write_1h_price).toBe(10)
    expect(row.cache_read_price).toBe(0.5)
    expect(row.display_input_price).toBe(5)
    expect(row.display_output_price).toBe(25)
    expect(row.display_cache_creation_price).toBe(6.25)
    expect(row.display_cache_creation_1h_price).toBe(10)
    expect(row.display_cache_read_price).toBe(0.5)
  })

  it('adds missing curated models then applies suggested only to that set', () => {
    const existing = emptyOverrideRow('claude-opus-4-8', 'anthropic')
    existing.notes = 'keep-me'
    existing.enabled = false
    const rows = ensureCuratedOverrideRows([existing], ['claude-opus-4-8', 'claude-sonnet-4-6'], 'anthropic')
    expect(rows).toHaveLength(2)
    expect(rows[1].model).toBe('claude-sonnet-4-6')

    applySuggestedToCurated(rows, ['claude-opus-4-8', 'claude-sonnet-4-6'], (model, field) => {
      if (model === 'claude-opus-4-8') return litellmSuggestedMTok(source, field)
      if (field === 'input_price') return 3
      return null
    })

    expect(rows[0].notes).toBe('keep-me')
    expect(rows[0].enabled).toBe(false)
    expect(rows[0].input_price).toBe(5)
    expect(rows[0].display_input_price).toBe(5)
    expect(rows[1].input_price).toBe(3)
    expect(rows[1].display_input_price).toBe(3)
    expect(rows[1].output_price).toBeNull()
  })
})
