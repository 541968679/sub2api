import { describe, expect, it } from 'vitest'
import type { AccountQualityStats } from '@/api/admin/accounts'
import type { UserSmartScheduleView } from '@/api/admin/users'
import {
  POOL_ADMISSION_FILTER_STATES,
  isCurrentlySchedulingAccount,
  matchesPoolFilters,
  pickDefaultSmartSchedulePlatform,
  resolvePairCap,
  resolvePoolAdmission,
  resolveQualityAdmissionHint,
  userQualityResumeActive,
  userQualityResumeChipActive
} from '../smartSchedulePoolAdmission'

function emptyPlatform(overrides: Partial<UserSmartScheduleView['platforms'][string]> = {}) {
  return {
    enabled: false,
    quality_max_p50_ttft_ms: null,
    quality_min_success_rate: null,
    quality_min_success_samples: null,
    quality_min_ttft_samples: null,
    quality_condition: null,
    cooldown_minutes: 15,
    accounts: [],
    ...overrides
  }
}

const live = { status: 'active' as const, schedulable: true }

const failingStats: AccountQualityStats = {
  window_seconds: 900,
  success_count: 0,
  error_count: 8,
  success_rate: 0,
  p50_ttft_ms: 900,
  ttft_samples: 8
}

const passingStats: AccountQualityStats = {
  window_seconds: 900,
  success_count: 20,
  error_count: 0,
  success_rate: 1,
  p50_ttft_ms: 80,
  ttft_samples: 20
}

const savedLiveGate = {
  enabled: true,
  maxP50: 200,
  successPercent: 90,
  minSuccessSamples: 1,
  minTtftSamples: 1,
  condition: 'or' as const
}

const looseSavedGate = {
  enabled: true,
  maxP50: 2000,
  successPercent: 10,
  minSuccessSamples: 1,
  minTtftSamples: 1,
  condition: 'or' as const
}

describe('resolvePairCap', () => {
  it('treats empty and zero as no extra pair cap', () => {
    expect(resolvePairCap(null)).toBeNull()
    expect(resolvePairCap(undefined)).toBeNull()
    expect(resolvePairCap(0)).toBeNull()
    expect(resolvePairCap(3)).toBe(3)
  })
})

describe('resolvePoolAdmission', () => {
  it('marks account-level stopped scheduling first', () => {
    expect(
      resolvePoolAdmission({
        account: { status: 'active', schedulable: false },
        pairCap: 2,
        pairCurrent: 2,
        cooldownUntil: new Date(Date.now() + 60_000).toISOString(),
        qualityHint: 'will_cool'
      }).state
    ).toBe('stopped')
  })

  it('marks pair cooldown as the only quality lock', () => {
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: 1,
        pairCurrent: 1,
        cooldownUntil: new Date(Date.now() + 60_000).toISOString(),
        qualityHint: 'will_cool'
      }).state
    ).toBe('cooling')
  })

  it('after cooldown expires, a saved-gate miss is will-cool not a lock', () => {
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: null,
        pairCurrent: 0,
        cooldownUntil: new Date(Date.now() - 60_000).toISOString(),
        qualityHint: 'will_cool'
      }).state
    ).toBe('will_cool')
  })

  it('marks pair-full only when a real pair cap exists', () => {
    expect(resolvePoolAdmission({ account: live, pairCap: null, pairCurrent: 9 }).state).toBe('selectable')
    expect(resolvePoolAdmission({ account: live, pairCap: 2, pairCurrent: 2 }).state).toBe('pair_full')
  })

  it('keeps resume and preview below pair-full', () => {
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: 1,
        pairCurrent: 1,
        qualityHint: 'resumed'
      }).state
    ).toBe('pair_full')
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: null,
        pairCurrent: 0,
        qualityHint: 'resumed'
      }).state
    ).toBe('resumed')
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: null,
        pairCurrent: 0,
        qualityHint: 'unsaved_preview'
      }).state
    ).toBe('unsaved_preview')
  })
})

describe('resolveQualityAdmissionHint', () => {
  it('labels a saved live-gate miss as will-cool', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        stats: failingStats
      })
    ).toBe('will_cool')
  })

  it('labels a tighter unsaved draft as preview when the saved gate still passes', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: looseSavedGate,
        stats: {
          window_seconds: 900,
          success_count: 8,
          error_count: 2,
          success_rate: 0.8,
          p50_ttft_ms: 400,
          ttft_samples: 10
        }
      })
    ).toBe('unsaved_preview')
  })

  it('labels a disabled-platform gate miss as preview, not will-cool', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: { ...savedLiveGate, enabled: false },
        saved: { ...savedLiveGate, enabled: false },
        stats: failingStats
      })
    ).toBe('unsaved_preview')
  })

  it('keeps the live will-cool hint when the saved gate already fails and the draft is dirty', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: { ...savedLiveGate, maxP50: 50 },
        saved: savedLiveGate,
        stats: failingStats
      })
    ).toBe('will_cool')
  })

  it('shows resumed while the chip grace is active', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        stats: failingStats,
        resumeChipActive: true,
        resumeActive: true
      })
    ).toBe('resumed')
  })

  it('stays selectable during watching grace so stats cannot fake a lock', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        stats: failingStats,
        resumeActive: true
      })
    ).toBeNull()
  })

  it('does not hint when the saved live gate still passes', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        stats: passingStats
      })
    ).toBeNull()
  })
})

describe('userQualityResumeActive', () => {
  const now = Date.parse('2026-08-16T12:00:00.000Z')

  it('reads resume_users and resume_watching_users when batch exposes them', () => {
    expect(
      userQualityResumeChipActive({ resume_users: { '99': now / 1000 + 60 } }, 99, now)
    ).toBe(true)
    expect(
      userQualityResumeActive({ resume_watching_users: { '99': now / 1000 + 60 } }, 99, now)
    ).toBe(true)
    expect(userQualityResumeActive({ resume_users: { '99': now / 1000 - 1 } }, 99, now)).toBe(false)
  })
})

describe('POOL_ADMISSION_FILTER_STATES', () => {
  it('does not treat a quality miss as a dead lock filter', () => {
    expect(POOL_ADMISSION_FILTER_STATES).toContain('will_cool')
    expect(POOL_ADMISSION_FILTER_STATES).toContain('unsaved_preview')
    expect(POOL_ADMISSION_FILTER_STATES).toContain('resumed')
    expect(POOL_ADMISSION_FILTER_STATES as readonly string[]).not.toContain('quality_blocked')
  })
})

describe('matchesPoolFilters', () => {
  const account = {
    id: 7,
    name: 'oauth-bot',
    type: 'oauth',
    schedulable: false,
    extra: { email_address: 'a@example.com' }
  } as any

  it('filters type, schedulable, admission, and search together', () => {
    expect(
      matchesPoolFilters(account, 'stopped', {
        search: 'oauth',
        type: 'oauth',
        schedulable: 'off',
        admission: 'stopped'
      })
    ).toBe(true)
    expect(
      matchesPoolFilters(account, 'will_cool', {
        search: '',
        type: 'oauth',
        schedulable: 'off',
        admission: 'stopped'
      })
    ).toBe(false)
  })
})

describe('pickDefaultSmartSchedulePlatform', () => {
  it('uses the backend hint when valid', () => {
    expect(
      pickDefaultSmartSchedulePlatform({
        user_id: 16,
        default_platform: 'openai',
        platforms: {
          anthropic: emptyPlatform(),
          openai: emptyPlatform({ enabled: true, accounts: [{ account_id: 1 }] })
        }
      })
    ).toBe('openai')
  })

  it('prefers the only enabled platform for zuoge85-style users', () => {
    expect(
      pickDefaultSmartSchedulePlatform({
        user_id: 16,
        platforms: {
          anthropic: emptyPlatform(),
          openai: emptyPlatform({ enabled: true, accounts: [{ account_id: 1 }, { account_id: 2 }] }),
          gemini: emptyPlatform(),
          antigravity: emptyPlatform(),
          grok: emptyPlatform()
        }
      })
    ).toBe('openai')
  })
})

describe('isCurrentlySchedulingAccount', () => {
  it('accepts active schedulable accounts', () => {
    expect(isCurrentlySchedulingAccount({ status: 'active', schedulable: true })).toBe(true)
  })
})
