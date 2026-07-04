import { describe, expect, it } from 'vitest'
import { deriveModelNameRows } from '../modelPricingRows'
import type { ModelPricingItem } from '@/api/admin/modelPricing'

function makeItem(overrides: Partial<ModelPricingItem>): ModelPricingItem {
  return {
    model: 'model-x',
    provider: 'anthropic',
    litellm_prices: null,
    global_override: null,
    channel_override_count: 0,
    effective_source: 'fallback',
    ...overrides,
  }
}

describe('deriveModelNameRows', () => {
  it('renders a pass-through row when the model has no mapping entry', () => {
    const rows = deriveModelNameRows(makeItem({ model: 'claude-opus-4-5' }))
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      mappingFrom: 'claude-opus-4-5',
      upstreamDisplay: 'claude-opus-4-5',
      isMappingEntry: false,
      mappedFrom: [],
    })
  })

  it('renders the mapping entry row for a requested_only hint', () => {
    const rows = deriveModelNameRows(
      makeItem({
        model: 'fable-5-vvip',
        billing_basis_hints: [
          {
            platform: 'anthropic',
            type: 'requested_only',
            mapping_key: 'fable-5-vvip',
            mapping_target: 'claude-fable-5',
            related_models: ['claude-fable-5'],
          },
        ],
      })
    )
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      mappingFrom: 'fable-5-vvip',
      upstreamDisplay: 'claude-fable-5',
      platform: 'anthropic',
      isMappingEntry: true,
    })
  })

  // bug 回归：模型既是映射键又是其他键的映射目标时（a -> b 且 b -> c），
  // b 的行必须保留自己的映射目标 c，而不是被"来源展开"覆盖成 b -> b。
  it('keeps the mapping target when the model is also a mapping target of other keys', () => {
    const rows = deriveModelNameRows(
      makeItem({
        model: 'claude-sonnet-4-5',
        billing_basis_hints: [
          {
            platform: 'antigravity',
            type: 'requested_only',
            mapping_key: 'claude-sonnet-4-5',
            mapping_target: 'claude-fable-5',
            mapped_from: ['claude-sonnet-4-5-20250929'],
          },
        ],
      })
    )
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      mappingFrom: 'claude-sonnet-4-5',
      upstreamDisplay: 'claude-fable-5',
      isMappingEntry: true,
      mappedFrom: ['claude-sonnet-4-5-20250929'],
    })
  })

  // bug 回归：纯映射目标（如 claude-fable-5 被 fable-5-vvip 指向）必须保留
  // 自己的直通行，不能被展开成映射源的行导致该请求模型从表里消失。
  it('keeps an own pass-through row for a pure mapping target', () => {
    const rows = deriveModelNameRows(
      makeItem({
        model: 'claude-fable-5',
        billing_basis_hints: [
          {
            platform: 'anthropic',
            type: 'upstream_only',
            mapping_key: 'fable-5-vvip',
            mapped_from: ['fable-5-vvip'],
            related_models: ['fable-5-vvip'],
          },
        ],
      })
    )
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      mappingFrom: 'claude-fable-5',
      upstreamDisplay: 'claude-fable-5',
      isMappingEntry: false,
      mappedFrom: ['fable-5-vvip'],
    })
  })

  it('renders a same-name mapping entry as an editable row', () => {
    const rows = deriveModelNameRows(
      makeItem({
        model: 'claude-sonnet-4-6',
        billing_basis_hints: [
          {
            platform: 'antigravity',
            type: 'requested_equals_upstream',
            mapping_key: 'claude-sonnet-4-6',
            mapping_target: 'claude-sonnet-4-6',
          },
        ],
      })
    )
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      mappingFrom: 'claude-sonnet-4-6',
      upstreamDisplay: 'claude-sonnet-4-6',
      isMappingEntry: true,
    })
  })

  it('renders one row per platform when the model is mapped on multiple platforms', () => {
    const rows = deriveModelNameRows(
      makeItem({
        model: 'claude-haiku-4-5',
        billing_basis_hints: [
          {
            platform: 'antigravity',
            type: 'requested_only',
            mapping_key: 'claude-haiku-4-5',
            mapping_target: 'gemini-2.5-flash',
          },
          {
            platform: 'anthropic',
            type: 'requested_only',
            mapping_key: 'claude-haiku-4-5',
            mapping_target: 'claude-3-haiku-20240307',
          },
        ],
      })
    )
    expect(rows).toHaveLength(2)
    expect(rows.map((r) => r.rowKey)).toEqual([
      'antigravity:claude-haiku-4-5',
      'anthropic:claude-haiku-4-5',
    ])
    expect(rows.map((r) => r.upstreamDisplay)).toEqual([
      'gemini-2.5-flash',
      'claude-3-haiku-20240307',
    ])
  })

  it('resolves the billing object per mapping key', () => {
    const rows = deriveModelNameRows(
      makeItem({
        model: 'claude-opus-4-8',
        billing_basis_hints: [
          {
            platform: 'antigravity',
            type: 'requested_only',
            mapping_key: 'claude-opus-4-8',
            mapping_target: 'claude-opus-4-6-thinking',
            billing_object: 'mapped',
            mapping_billing_objects: { 'claude-opus-4-8': 'mapped' },
          },
        ],
      })
    )
    expect(rows[0].billingObject).toBe('mapped')
  })

  it('falls back to the legacy single hint when hints array is absent', () => {
    const rows = deriveModelNameRows(
      makeItem({
        model: 'old-model',
        billing_basis_hint: {
          platform: 'openai',
          type: 'requested_only',
          mapping_key: 'old-model',
          mapping_target: 'gpt-5.5',
        },
      })
    )
    expect(rows[0]).toMatchObject({
      mappingFrom: 'old-model',
      upstreamDisplay: 'gpt-5.5',
      platform: 'openai',
      isMappingEntry: true,
    })
  })
})
