import { describe, expect, it } from 'vitest'
import {
  isCurrentlySchedulingAccount,
  matchesPoolFilters,
  pickDefaultSmartSchedulePlatform,
  resolvePairCap,
  resolvePoolAdmission
} from '../smartSchedulePoolAdmission'
import type { UserSmartScheduleView } from '@/api/admin/users'

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

describe('resolvePairCap', () => {
  it('treats empty and zero as no extra pair cap', () => {
    expect(resolvePairCap(null)).toBeNull()
    expect(resolvePairCap(undefined)).toBeNull()
    expect(resolvePairCap(0)).toBeNull()
    expect(resolvePairCap(3)).toBe(3)
  })
})

describe('resolvePoolAdmission', () => {
  const live = { status: 'active' as const, schedulable: true }

  it('marks account-level stopped scheduling first', () => {
    expect(
      resolvePoolAdmission({
        account: { status: 'active', schedulable: false },
        pairCap: 2,
        pairCurrent: 2,
        cooldownUntil: new Date(Date.now() + 60_000).toISOString()
      }).state
    ).toBe('stopped')
  })

  it('marks pair cooldown before pair-full', () => {
    expect(
      resolvePoolAdmission({
        account: live,
        pairCap: 1,
        pairCurrent: 1,
        cooldownUntil: new Date(Date.now() + 60_000).toISOString()
      }).state
    ).toBe('cooling')
  })

  it('marks pair-full only when a real pair cap exists', () => {
    expect(resolvePoolAdmission({ account: live, pairCap: null, pairCurrent: 9 }).state).toBe('selectable')
    expect(resolvePoolAdmission({ account: live, pairCap: 2, pairCurrent: 2 }).state).toBe('pair_full')
  })

  it('marks quality-blocked when the live gate is breached', () => {
    expect(resolvePoolAdmission({ account: live, pairCap: null, pairCurrent: 0, qualityBlocked: true }).state).toBe(
      'quality_blocked'
    )
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
      matchesPoolFilters(account, 'stopped', {
        search: '',
        type: 'apikey',
        schedulable: 'off',
        admission: 'stopped'
      })
    ).toBe(false)
    expect(
      matchesPoolFilters(account, 'selectable', {
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
