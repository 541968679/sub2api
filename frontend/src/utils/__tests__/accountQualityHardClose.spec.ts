import { describe, expect, it } from 'vitest'
import {
  applyQualityGateFormToDraft,
  mergeQualityTemplateFromGate,
  qualityGateFormFromDraft,
  qualityGateFormFromTemplate,
  scheduleUserQualityChipState
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
      quality_min_ttft_samples: 8,
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
      account_quality_window_n: 8,
      min_success_samples: 8,
      min_ttft_samples: 8,
      condition: 'or',
      schedule_use_failover_error_rate: false
    })
  })

  it('round-trips an inline draft through the shared form shape', () => {
    const draft = {
      maxP50: '1600',
      successPercent: '90',
      windowN: '10',
      condition: 'or' as const
    }
    const fields = qualityGateFormFromDraft(draft)
    expect(fields).toEqual({
      quality_max_p50_ttft_ms: 1600,
      quality_min_success_rate_percent: 90,
      quality_min_success_samples: 10,
      quality_min_ttft_samples: 10,
      quality_condition: 'or'
    })
    applyQualityGateFormToDraft(draft, qualityGateFormFromTemplate(template))
    expect(draft).toEqual({
      maxP50: 1800,
      successPercent: 95,
      windowN: 8,
      condition: 'and'
    })
  })
})

describe('scheduleUserQualityChipState', () => {
  const gate = { quality_max_p50_ttft_ms: 1500 }
  const now = Date.parse('2026-08-14T12:00:00Z')
  const breached = { p50_ttft_ms: 4000, ttft_samples: 12, success_count: 20, error_count: 0, success_rate: 1 }

  it('shows 已恢复 while the resume chip phase is active', () => {
    expect(scheduleUserQualityChipState({
      ...gate,
      quality_resumed_until: now / 1000 + 900,
      quality_window_until: now / 1000 + 1800,
      quality_blocked: true
    }, breached, now)).toBe('resumed')
  })

  it('returns to 质量 after 15 minutes and ignores the old breached window', () => {
    expect(scheduleUserQualityChipState({
      ...gate,
      quality_resumed_until: now / 1000 + 900,
      quality_window_until: now / 1000 + 1800
    }, breached, now + 901_000)).toBe('configured')
  })

  it('returns to 质量 immediately after clicking 已恢复', () => {
    expect(scheduleUserQualityChipState({
      ...gate,
      quality_resumed_until: null,
      quality_window_until: now / 1000 + 900
    }, breached, now)).toBe('configured')
  })

  it('shows 已停 when the live window is still blocking', () => {
    expect(scheduleUserQualityChipState({
      ...gate,
      quality_blocked: true
    }, breached, now)).toBe('blocked')
  })
})
