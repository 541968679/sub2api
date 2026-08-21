import { describe, expect, it } from 'vitest'
import type { UserSmartScheduleView } from '@/api/admin/users'
import { normalizeSmartSchedulePairQuality, resolveSmartScheduleWindowN } from '@/utils/smartScheduleWindowN'
import {
  POOL_ADMISSION_FILTER_STATES,
  isCurrentlySchedulingAccount,
  matchesPoolFilters,
  pickDefaultSmartSchedulePlatform,
  pairOccupancyDisplayMax,
  pairOccupancyDisplayMaxForAdmission,
  pairQualityGateBreached,
  readBackendProbeCap,
  resolvePairCap,
  resolvePoolAdmission,
  resolveProbeConcurrency,
  UNCAPPED_PAIR_DISPLAY_MAX,
  resolveQualityAdmissionHint,
  pairAdmissionLiveState,
  PAIR_ADMISSION_LIVE_STATES,
  memberProbingFromApi,
  userQualityResumeActive,
  userQualityResumeChipActive
} from '../smartSchedulePoolAdmission'

function emptyPlatform(overrides: Partial<UserSmartScheduleView['platforms'][string]> = {}) {
  return {
    enabled: false,
    quality_max_p50_ttft_ms: null,
    quality_min_success_rate: null,
    quality_window_n: null,
    quality_min_success_samples: null,
    quality_min_ttft_samples: null,
    quality_condition: null,
    cooldown_minutes: 15,
    accounts: [],
    ...overrides
  }
}

const live = { status: 'active' as const, schedulable: true }

const failingPair = {
  ttft_p50_ms: 900,
  success_rate: 0,
  ttft_samples: 8,
  ok_samples: 8,
  n: 1
}

const passingPair = {
  ttft_p50_ms: 80,
  success_rate: 1,
  ttft_samples: 20,
  ok_samples: 20,
  n: 10
}

const savedLiveGate = {
  enabled: true,
  maxP50: 200,
  successPercent: 90,
  windowN: 1,
  condition: 'or' as const
}

const looseSavedGate = {
  enabled: true,
  maxP50: 2000,
  successPercent: 10,
  windowN: 1,
  condition: 'or' as const
}

describe('resolveSmartScheduleWindowN', () => {
  it('prefers the new field, then legacy min-sample echoes, then default 10', () => {
    expect(resolveSmartScheduleWindowN({ quality_window_n: 7 })).toBe(7)
    expect(resolveSmartScheduleWindowN({ quality_window_samples: 9 })).toBe(9)
    expect(
      resolveSmartScheduleWindowN({
        quality_window_n: 12,
        quality_window_samples: 9,
        quality_min_success_samples: 3,
        quality_min_ttft_samples: 4
      })
    ).toBe(12)
    expect(
      resolveSmartScheduleWindowN({
        quality_min_success_samples: 8,
        quality_min_ttft_samples: 8
      })
    ).toBe(8)
    expect(
      resolveSmartScheduleWindowN({
        quality_min_success_samples: 20,
        quality_min_ttft_samples: 10
      })
    ).toBe(10)
    expect(resolveSmartScheduleWindowN({ quality_min_ttft_samples: 6 })).toBe(6)
    expect(resolveSmartScheduleWindowN({})).toBe(10)
    expect(resolveSmartScheduleWindowN({ quality_window_n: 0 })).toBe(1)
    expect(resolveSmartScheduleWindowN({ quality_window_n: 250 })).toBe(100)
  })
})

describe('normalizeSmartSchedulePairQuality', () => {
  it('reads backend canonical counts and p50 aliases', () => {
    expect(
      normalizeSmartSchedulePairQuality({
        p50_ttft_ms: 120,
        success_rate: 0.9,
        ttft_count: 4,
        ok_count: 6,
        n: 10
      })
    ).toEqual({
      ttft_p50_ms: 120,
      success_rate: 0.9,
      ttft_samples: 4,
      ok_samples: 6,
      n: 10
    })
  })
})

describe('resolvePairCap', () => {
  it('treats empty and zero as no extra pair cap', () => {
    expect(resolvePairCap(null)).toBeNull()
    expect(resolvePairCap(undefined)).toBeNull()
    expect(resolvePairCap(0)).toBeNull()
    expect(resolvePairCap(3)).toBe(3)
  })
})

describe('pairOccupancyDisplayMax', () => {
  it('uses 999 only as the uncapped display denominator', () => {
    expect(pairOccupancyDisplayMax(null)).toBe(UNCAPPED_PAIR_DISPLAY_MAX)
    expect(pairOccupancyDisplayMax(0)).toBe(UNCAPPED_PAIR_DISPLAY_MAX)
    expect(pairOccupancyDisplayMax(undefined)).toBe(UNCAPPED_PAIR_DISPLAY_MAX)
    expect(pairOccupancyDisplayMax(2)).toBe(2)
    expect(UNCAPPED_PAIR_DISPLAY_MAX).toBe(999)
  })
})

describe('resolveProbeConcurrency', () => {
  it('uses the backend field when present, else min(N, cap) or N', () => {
    expect(resolveProbeConcurrency({ windowN: 10, pairCap: 3, backendCap: 2 })).toBe(2)
    expect(resolveProbeConcurrency({ windowN: 10, pairCap: 3 })).toBe(3)
    expect(resolveProbeConcurrency({ windowN: 10, pairCap: 20 })).toBe(10)
    expect(resolveProbeConcurrency({ windowN: 10, pairCap: null })).toBe(10)
    expect(resolveProbeConcurrency({ windowN: 10, pairCap: 0 })).toBe(10)
    expect(readBackendProbeCap({ probing_cap: 4 })).toBe(4)
    expect(readBackendProbeCap({ in_flight_cap: 6, probe_cap: 5 })).toBe(5)
  })
})

describe('pairOccupancyDisplayMaxForAdmission', () => {
  it('never uses 999 as the probe denominator', () => {
    expect(
      pairOccupancyDisplayMaxForAdmission({ probing: true, pairCap: null, windowN: 10 })
    ).toBe(10)
    expect(
      pairOccupancyDisplayMaxForAdmission({ probing: true, pairCap: 3, windowN: 10 })
    ).toBe(3)
    expect(
      pairOccupancyDisplayMaxForAdmission({ probing: false, pairCap: null, windowN: 10 })
    ).toBe(UNCAPPED_PAIR_DISPLAY_MAX)
  })
})

describe('memberProbingFromApi', () => {
  it('treats a missing mark as not probing (no backfill)', () => {
    expect(memberProbingFromApi(undefined)).toBe(false)
    expect(memberProbingFromApi({})).toBe(false)
    expect(memberProbingFromApi({ probing: true })).toBe(true)
    expect(memberProbingFromApi({ admission: 'probing' })).toBe(true)
    expect(memberProbingFromApi({ state: 'selectable' })).toBe(false)
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

  it('marks a durable pair pause after account-level stopped', () => {
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: 1,
        pairCurrent: 1,
        paused: true,
        cooldownUntil: new Date(Date.now() + 60_000).toISOString(),
        qualityHint: 'resumed'
      }).state
    ).toBe('paused')
    expect(
      resolvePoolAdmission({
        account: { status: 'active', schedulable: false },
        pairCap: null,
        pairCurrent: 0,
        paused: true
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

  it('keeps probing after cooldown expires, and does not invent it from a missing mark', () => {
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: 2,
        pairCurrent: 2,
        probing: true,
        qualityHint: 'will_cool'
      }).state
    ).toBe('probing')
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: null,
        pairCurrent: 0,
        cooldownUntil: new Date(Date.now() - 60_000).toISOString()
      }).state
    ).toBe('selectable')
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: null,
        pairCurrent: 0,
        paused: true,
        probing: true
      }).state
    ).toBe('paused')
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

describe('pairQualityGateBreached', () => {
  it('does not cool when a window is still under N', () => {
    expect(
      pairQualityGateBreached(savedLiveGate, {
        ttft_p50_ms: 900,
        success_rate: 0,
        ttft_samples: 0,
        ok_samples: 0,
        n: 10
      })
    ).toBe(false)
    expect(
      pairQualityGateBreached({ ...savedLiveGate, windowN: 10 }, {
        ttft_p50_ms: 900,
        success_rate: 0,
        ttft_samples: 9,
        ok_samples: 9,
        n: 10
      })
    ).toBe(false)
  })

  it('cools from pair windows once N is met, not from missing pair data', () => {
    expect(pairQualityGateBreached(savedLiveGate, failingPair)).toBe(true)
    expect(pairQualityGateBreached(savedLiveGate, null)).toBe(false)
  })
})

describe('resolveQualityAdmissionHint', () => {
  it('labels a saved live-gate miss as will-cool from pair windows', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        pairQuality: failingPair
      })
    ).toBe('will_cool')
  })

  it('ignores leftover account 15m cells on the deprecated stats field', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        stats: {
          window_seconds: 900,
          success_count: 0,
          error_count: 8,
          success_rate: 0,
          p50_ttft_ms: 900,
          ttft_samples: 8
        }
      })
    ).toBeNull()
  })

  it('labels a tighter unsaved draft as preview when the saved gate still passes', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: looseSavedGate,
        pairQuality: {
          ttft_p50_ms: 400,
          success_rate: 0.8,
          ttft_samples: 10,
          ok_samples: 10,
          n: 1
        }
      })
    ).toBe('unsaved_preview')
  })

  it('labels a disabled-platform gate miss as preview, not will-cool', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: { ...savedLiveGate, enabled: false },
        saved: { ...savedLiveGate, enabled: false },
        pairQuality: failingPair
      })
    ).toBe('unsaved_preview')
  })

  it('keeps the live will-cool hint when the saved gate already fails and the draft is dirty', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: { ...savedLiveGate, maxP50: 50 },
        saved: savedLiveGate,
        pairQuality: failingPair
      })
    ).toBe('will_cool')
  })

  it('shows 豁免期 while the chip or remaining watching window is active', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        pairQuality: failingPair,
        resumeChipActive: true,
        resumeActive: true
      })
    ).toBe('resumed')
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        pairQuality: failingPair,
        resumeActive: true
      })
    ).toBe('resumed')
  })

  it('does not fail-open selectable just because pair quality is missing', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate
      })
    ).toBeNull()
  })

  it('does not hint when the saved live pair window still passes', () => {
    expect(
      resolveQualityAdmissionHint({
        draft: savedLiveGate,
        saved: savedLiveGate,
        pairQuality: passingPair
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

describe('pairAdmissionLiveState', () => {
  it('maps display admission to the five writable live states', () => {
    expect(PAIR_ADMISSION_LIVE_STATES).toEqual(['paused', 'cooling', 'probing', 'selectable', 'resumed'])
    expect(pairAdmissionLiveState('paused')).toBe('paused')
    expect(pairAdmissionLiveState('cooling')).toBe('cooling')
    expect(pairAdmissionLiveState('probing')).toBe('probing')
    expect(pairAdmissionLiveState('resumed')).toBe('resumed')
    expect(pairAdmissionLiveState('selectable')).toBe('selectable')
    expect(pairAdmissionLiveState('will_cool')).toBe('selectable')
    expect(pairAdmissionLiveState('unsaved_preview')).toBe('selectable')
    expect(pairAdmissionLiveState('pair_full')).toBe('selectable')
    expect(pairAdmissionLiveState('stopped')).toBe('selectable')
    expect(pairAdmissionLiveState('stopped', true)).toBe('paused')
    expect(pairAdmissionLiveState('selectable', true)).toBe('paused')
    expect(pairAdmissionLiveState('probing', true)).toBe('paused')
  })
})

describe('POOL_ADMISSION_FILTER_STATES', () => {
  it('does not treat a quality miss as a dead lock filter', () => {
    expect(POOL_ADMISSION_FILTER_STATES).toContain('will_cool')
    expect(POOL_ADMISSION_FILTER_STATES).toContain('unsaved_preview')
    expect(POOL_ADMISSION_FILTER_STATES).toContain('resumed')
    expect(POOL_ADMISSION_FILTER_STATES).toContain('probing')
    expect(POOL_ADMISSION_FILTER_STATES).toContain('paused')
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
