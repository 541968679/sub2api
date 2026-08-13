import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountUserScheduleCell from '../AccountUserScheduleCell.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: { count?: number }) => {
        if (key === 'admin.accounts.userSchedule.userCountTotal') {
          return `count:${params?.count ?? 0}`
        }
        return key
      }
    })
  }
})

function makeAccount(overrides: Partial<Account> = {}): Account {
  return {
    id: 1,
    name: 'account',
    platform: 'anthropic',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-08-13T00:00:00Z',
    updated_at: '2026-08-13T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  }
}

describe('AccountUserScheduleCell', () => {
  it('unrestricted 显示 —', () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: { account: makeAccount({ user_schedule_mode: 'unrestricted' }) }
    })
    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).not.toContain('admin.accounts.userSchedule.modeAllow')
  })

  it('allow 显示模式标签和用户邮箱', () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          user_schedule_mode: 'allow',
          schedule_users: [{ id: 16, email: 'a@x.com', deleted: false }]
        })
      }
    })
    expect(wrapper.text()).toContain('admin.accounts.userSchedule.modeAllow')
    expect(wrapper.text()).toContain('a@x.com')
  })

  it('用户过多时显示 +N', () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          user_schedule_mode: 'deny',
          schedule_users: [
            { id: 1, email: 'a@x.com', deleted: false },
            { id: 2, email: 'b@x.com', deleted: false },
            { id: 3, email: 'c@x.com', deleted: false },
            { id: 4, email: 'd@x.com', deleted: false },
            { id: 5, email: 'e@x.com', deleted: false }
          ]
        }),
        maxDisplay: 4
      }
    })
    expect(wrapper.text()).toContain('+2')
    expect(wrapper.text()).toContain('admin.accounts.userSchedule.modeDeny')
  })
})
