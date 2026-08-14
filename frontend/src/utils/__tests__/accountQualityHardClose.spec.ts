import { describe, expect, it } from 'vitest'
import {
  applyQualityGateFormToDraft,
  mergeQualityTemplateFromGate,
  qualityGateFormFromDraft,
  qualityGateFormFromTemplate
} from '@/utils/accountQualityHardClose'

const template = {
  enabled: true,
  max_p50_ttft_ms: 1800,
  min_success_rate: 0.95,
  pause_minutes: 15,
  min_success_samples: 8,
  min_ttft_samples: 6,
  condition: 'and' as const
}

describe('accountQualityHardClose template helpers', () => {
  it('maps the shared template into a user-gate form without pause or enabled', () => {
    expect(qualityGateFormFromTemplate(template)).toEqual({
      quality_max_p50_ttft_ms: 1800,
      quality_min_success_rate_percent: 95,
      quality_min_success_samples: 8,
      quality_min_ttft_samples: 6,
      quality_condition: 'and'
    })
  })

  it('saves gate fields over the shared template without binding a user or flipping the switch', () => {
    expect(mergeQualityTemplateFromGate(template, {
      quality_max_p50_ttft_ms: 2500,
      quality_min_success_rate_percent: 85,
      quality_min_success_samples: null,
      quality_min_ttft_samples: 9,
      quality_condition: 'or'
    })).toEqual({
      enabled: true,
      max_p50_ttft_ms: 2500,
      min_success_rate: 0.85,
      pause_minutes: 15,
      min_success_samples: 8,
      min_ttft_samples: 9,
      condition: 'or'
    })
  })

  it('round-trips an inline draft through the shared form shape', () => {
    const draft = {
      maxP50: '1600',
      successPercent: '90',
      minSuccessSamples: '',
      minTtftSamples: '10',
      condition: 'or' as const
    }
    const fields = qualityGateFormFromDraft(draft)
    expect(fields).toEqual({
      quality_max_p50_ttft_ms: 1600,
      quality_min_success_rate_percent: 90,
      quality_min_success_samples: null,
      quality_min_ttft_samples: 10,
      quality_condition: 'or'
    })
    applyQualityGateFormToDraft(draft, qualityGateFormFromTemplate(template))
    expect(draft).toEqual({
      maxP50: 1800,
      successPercent: 95,
      minSuccessSamples: 8,
      minTtftSamples: 6,
      condition: 'and'
    })
  })
})
