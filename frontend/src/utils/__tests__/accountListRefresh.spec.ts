import { describe, expect, it } from 'vitest'
import type { Account, AccountScheduleUser } from '@/types'
import { scheduleUsersRefreshKey, shouldReplaceAccountListRow } from '../accountListRefresh'

const user = (overrides: Partial<AccountScheduleUser> = {}): AccountScheduleUser => ({
  id: 16,
  email: 'zuoge85@gmail.com',
  deleted: false,
  allow: true,
  quality_max_p50_ttft_ms: 4000,
  quality_min_success_rate: 0.95,
  quality_min_success_samples: 15,
  quality_min_ttft_samples: 10,
  quality_condition: 'or',
  ...overrides
})

const account = (overrides: Partial<Account> = {}): Account => ({
  id: 1689,
  name: 'aiwanwu 0.06',
  platform: 'openai',
  type: 'apikey',
  proxy_id: null,
  concurrency: 1,
  priority: 1,
  status: 'active',
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: true,
  created_at: '2026-08-15T00:00:00Z',
  updated_at: '2026-08-15T03:31:00Z',
  schedulable: true,
  rate_limited_at: null,
  rate_limit_reset_at: null,
  overload_until: null,
  temp_unschedulable_until: null,
  temp_unschedulable_reason: null,
  session_window_start: null,
  session_window_end: null,
  session_window_status: null,
  current_concurrency: 0,
  schedule_users: [user()],
  ...overrides
})

describe('scheduleUsersRefreshKey', () => {
  it('treats omitted quality_blocked as the same as false', () => {
    expect(scheduleUsersRefreshKey([user()])).toBe(
      scheduleUsersRefreshKey([user({ quality_blocked: false })])
    )
  })

  it('changes when the server stamps 已停', () => {
    expect(scheduleUsersRefreshKey([user()])).not.toBe(
      scheduleUsersRefreshKey([user({ quality_blocked: true })])
    )
  })

  it('changes when a resume/window overlay should be cleared', () => {
    const overlay = user({
      quality_blocked: false,
      quality_window_until: 1_781_000_000
    })
    expect(scheduleUsersRefreshKey([overlay])).not.toBe(scheduleUsersRefreshKey([user()]))
  })
})

describe('shouldReplaceAccountListRow', () => {
  it('keeps the row when only unrelated live fields stay the same', () => {
    const current = account()
    const next = account({ schedule_users: [user({ quality_blocked: false })] })
    expect(shouldReplaceAccountListRow(current, next)).toBe(false)
  })

  it('replaces the row when quality_blocked flips and updated_at does not', () => {
    const current = account({ schedule_users: [user()] })
    const next = account({ schedule_users: [user({ quality_blocked: true })] })
    expect(shouldReplaceAccountListRow(current, next)).toBe(true)
  })

  it('still replaces on concurrency changes', () => {
    expect(shouldReplaceAccountListRow(account(), account({ current_concurrency: 2 }))).toBe(true)
  })
})
