import { describe, expect, it } from 'vitest'
import {
  emptySmartScheduleAddFilters,
  filterAddCandidates,
  matchesAddCandidateFilters
} from '../smartScheduleAddCandidates'
import type { Account } from '@/types'

function account(overrides: Partial<Account> = {}): Account {
  return {
    id: 21,
    name: 'alpha-bot',
    platform: 'anthropic',
    type: 'apikey',
    status: 'active',
    schedulable: true,
    proxy_id: 8,
    concurrency: 1,
    priority: 0,
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '',
    updated_at: '',
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    extra: { email_address: 'a@example.com' },
    ...overrides
  } as Account
}

describe('matchesAddCandidateFilters', () => {
  it('reuses account-list type/status/group and adds schedulable, scheduling, proxy, and id/email search', () => {
    const row = account()
    const filters = {
      ...emptySmartScheduleAddFilters('anthropic'),
      type: 'apikey',
      status: 'active',
      schedulable: 'on' as const,
      scheduling: 'on' as const,
      proxy: '8',
      search: '21'
    }
    expect(matchesAddCandidateFilters(row, filters)).toBe(true)
    expect(matchesAddCandidateFilters(row, { ...filters, search: 'a@example.com' })).toBe(true)
    expect(matchesAddCandidateFilters(row, { ...filters, type: 'oauth' })).toBe(false)
    expect(matchesAddCandidateFilters(row, { ...filters, schedulable: 'off' })).toBe(false)
    expect(matchesAddCandidateFilters(row, { ...filters, scheduling: 'off' })).toBe(false)
    expect(matchesAddCandidateFilters(row, { ...filters, proxy: 'none' })).toBe(false)
  })

  it('keeps the platform lock and drops already-filtered-out peers', () => {
    const rows = [
      account({ id: 1, name: 'keep', type: 'oauth', proxy_id: null }),
      account({ id: 2, name: 'drop', type: 'apikey', platform: 'openai' })
    ]
    const matched = filterAddCandidates(rows, {
      ...emptySmartScheduleAddFilters('anthropic'),
      type: 'oauth'
    })
    expect(matched.map((item) => item.id)).toEqual([1])
  })
})
