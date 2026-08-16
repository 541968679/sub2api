import { describe, expect, it } from 'vitest'
import {
  ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE,
  ACCOUNT_UNGROUPED_GROUP_QUERY_VALUE,
  accountMatchesListFilters
} from '../accountListFilters'
import type { Account } from '@/types'

function account(overrides: Partial<Account> = {}): Account {
  return {
    id: 1,
    name: 'alpha-bot',
    platform: 'anthropic',
    type: 'apikey',
    status: 'active',
    schedulable: true,
    proxy_id: null,
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
    ...overrides
  } as Account
}

describe('accountMatchesListFilters', () => {
  it('matches type, group, privacy, and name search like the account list', () => {
    const row = account({
      group_ids: [3],
      extra: { privacy_mode: 'training_off' }
    })
    expect(
      accountMatchesListFilters(row, {
        type: 'apikey',
        group: '3',
        privacy_mode: 'training_off',
        search: 'alpha'
      })
    ).toBe(true)
    expect(accountMatchesListFilters(row, { type: 'oauth' })).toBe(false)
    expect(accountMatchesListFilters(row, { group: ACCOUNT_UNGROUPED_GROUP_QUERY_VALUE })).toBe(false)
    expect(accountMatchesListFilters(row, { privacy_mode: ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE })).toBe(false)
    expect(accountMatchesListFilters(row, { search: 'beta' })).toBe(false)
  })

  it('treats active as currently schedulable, not merely status=active', () => {
    const live = account()
    const paused = account({ schedulable: false })
    expect(accountMatchesListFilters(live, { status: 'active' })).toBe(true)
    expect(accountMatchesListFilters(paused, { status: 'active' })).toBe(false)
    expect(accountMatchesListFilters(paused, { status: 'unschedulable' })).toBe(true)
  })
})
