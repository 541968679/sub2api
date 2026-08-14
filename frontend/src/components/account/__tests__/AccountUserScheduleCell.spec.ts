import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountUserScheduleCell from '../AccountUserScheduleCell.vue'
import AccountInlineNumberCell from '../AccountInlineNumberCell.vue'
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
  it('三套皆空显示 —', () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: { account: makeAccount({ user_schedule_mode: 'unrestricted', schedule_users: [] }) }
    })
    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).not.toContain('admin.accounts.userSchedule.modeAllow')
  })

  it('allow 显示模式标签和用户邮箱', () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{ id: 16, email: 'a@x.com', deleted: false, allow: true }]
        })
      }
    })
    expect(wrapper.text()).toContain('admin.accounts.userSchedule.modeAllow')
    expect(wrapper.text()).toContain('a@x.com')
  })

  it('deny+cap 同时显示拒绝标签和数字', () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{ id: 16, email: 'denied@x.com', deleted: false, deny: true, max_concurrency: 5 }]
        })
      }
    })
    expect(wrapper.text()).toContain('admin.accounts.userSchedule.modeDeny')
    expect(wrapper.text()).toContain('denied@x.com')
    expect(wrapper.text()).toContain('5')
  })

  it('unrestricted + 仅 cap 用户仍显示 chip 和数字', () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          user_schedule_mode: 'unrestricted',
          schedule_users: [{ id: 7, email: 'cap@x.com', deleted: false, max_concurrency: 3 }]
        })
      }
    })
    expect(wrapper.text()).toContain('cap@x.com')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).not.toContain('admin.accounts.userSchedule.modeAllow')
    expect(wrapper.text()).not.toContain('admin.accounts.userSchedule.modeDeny')
  })

  it('标记数字时发出 save，清空数字时传 null', async () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{ id: 16, email: 'a@x.com', deleted: false, allow: true, max_concurrency: 2 }]
        })
      }
    })
    const cell = wrapper.findComponent(AccountInlineNumberCell)
    expect(cell.exists()).toBe(true)
    await cell.vm.$emit('save', 5)
    expect(wrapper.emitted('save')?.[0]?.[0]).toEqual({ userId: 16, maxConcurrency: 5 })
    await cell.vm.$emit('save', 0)
    expect(wrapper.emitted('save')?.[1]?.[0]).toEqual({ userId: 16, maxConcurrency: null })
  })

  it('用户过多时显示 +N', () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [
            { id: 1, email: 'a@x.com', deleted: false, deny: true },
            { id: 2, email: 'b@x.com', deleted: false, deny: true },
            { id: 3, email: 'c@x.com', deleted: false, deny: true },
            { id: 4, email: 'd@x.com', deleted: false, deny: true },
            { id: 5, email: 'e@x.com', deleted: false, deny: true }
          ]
        }),
        maxDisplay: 4
      }
    })
    expect(wrapper.text()).toContain('+2')
    expect(wrapper.text()).toContain('admin.accounts.userSchedule.modeDeny')
  })
})
