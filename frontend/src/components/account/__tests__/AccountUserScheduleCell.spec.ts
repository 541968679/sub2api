import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AccountUserScheduleCell from '../AccountUserScheduleCell.vue'
import AccountInlineNumberCell from '../AccountInlineNumberCell.vue'
import type { Account } from '@/types'

const {
  getQualityHardCloseSettings,
  updateQualityHardCloseSettings,
  showSuccess
} = vi.hoisted(() => ({
  getQualityHardCloseSettings: vi.fn(),
  updateQualityHardCloseSettings: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    settings: {
      getQualityHardCloseSettings,
      updateQualityHardCloseSettings
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError: vi.fn()
  })
}))

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

  it('从空值输入数字后 blur 发出 save', async () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{ id: 1, email: 'admin@x.com', deleted: false, deny: true }]
        })
      }
    })
    await wrapper.get('button').trigger('click')
    const input = wrapper.get('input[type="number"]')
    await input.setValue('5')
    await input.trigger('blur')
    expect(wrapper.emitted('save')?.[0]?.[0]).toEqual({ userId: 1, maxConcurrency: 5 })
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

  it('有质量门槛时显示 Q chip，保存和清除发出 saveQuality', async () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{
            id: 16,
            email: 'vip@x.com',
            deleted: false,
            allow: true,
            quality_max_p50_ttft_ms: 1500,
            quality_min_success_rate: 0.9,
            quality_min_success_samples: 20,
            quality_min_ttft_samples: 10,
            quality_condition: 'or'
          }]
        })
      }
    })
    expect(wrapper.find('[data-testid="user-schedule-quality-chip"]').exists()).toBe(true)
    await wrapper.get('[data-testid="user-schedule-quality-edit-16"]').trigger('click')
    expect(wrapper.find('[data-testid="user-schedule-quality-editor-16"]').exists()).toBe(true)
    await wrapper.get('[data-testid="user-schedule-quality-resume"]').trigger('click')
    expect(wrapper.emitted('resumeQuality')?.[0]?.[0]).toBe(16)
    await wrapper.get('[data-testid="user-schedule-quality-p50"]').setValue('1800')
    await wrapper.get('[data-testid="user-schedule-quality-save"]').trigger('click')
    expect(wrapper.emitted('saveQuality')?.[0]?.[0]).toEqual({
      user_id: 16,
      quality_max_p50_ttft_ms: 1800,
      quality_min_success_rate: 0.9,
      quality_min_success_samples: 20,
      quality_min_ttft_samples: 20,
      quality_condition: 'or'
    })

    await wrapper.get('[data-testid="user-schedule-quality-edit-16"]').trigger('click')
    await wrapper.get('[data-testid="user-schedule-quality-clear"]').trigger('click')
    expect(wrapper.emitted('saveQuality')?.[1]?.[0]).toEqual({
      user_id: 16,
      quality_max_p50_ttft_ms: null,
      quality_min_success_rate: null,
      quality_min_success_samples: null,
      quality_min_ttft_samples: null,
      quality_condition: null
    })
  })

  it('已停芯片在立即恢复后变成已恢复，再点已恢复发出 startQualityWindow', async () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{
            id: 16,
            email: 'vip@x.com',
            deleted: false,
            allow: true,
            quality_max_p50_ttft_ms: 1500,
            quality_blocked: true
          }]
        })
      }
    })
    expect(wrapper.find('[data-testid="user-schedule-quality-blocked-chip"]').exists()).toBe(true)

    await wrapper.setProps({
      account: makeAccount({
        schedule_users: [{
          id: 16,
          email: 'vip@x.com',
          deleted: false,
          allow: true,
          quality_max_p50_ttft_ms: 1500,
          quality_resumed_until: Math.floor(Date.now() / 1000) + 900,
          quality_window_until: Math.floor(Date.now() / 1000) + 1800
        }]
      })
    })
    await wrapper.get('[data-testid="user-schedule-quality-resumed-chip"]').trigger('click')
    expect(wrapper.emitted('startQualityWindow')?.[0]?.[0]).toBe(16)
  })

  it('无指标时不显示质量芯片，空保存按清除发出', async () => {
    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{
            id: 16,
            email: 'vip@x.com',
            deleted: false,
            allow: true,
            quality_condition: 'or'
          }]
        })
      }
    })
    expect(wrapper.find('[data-testid="user-schedule-quality-chip"]').exists()).toBe(false)
    await wrapper.get('[data-testid="user-schedule-quality-edit-16"]').trigger('click')
    await wrapper.get('[data-testid="user-schedule-quality-save"]').trigger('click')
    expect(wrapper.emitted('saveQuality')?.[0]?.[0]).toEqual({
      user_id: 16,
      quality_max_p50_ttft_ms: null,
      quality_min_success_rate: null,
      quality_min_success_samples: null,
      quality_min_ttft_samples: null,
      quality_condition: null
    })
  })

  it('applies the shared template into the current editor without saving the user gate', async () => {
    getQualityHardCloseSettings.mockReset()
    updateQualityHardCloseSettings.mockReset()
    showSuccess.mockReset()
    getQualityHardCloseSettings.mockResolvedValue({
      enabled: true,
      max_p50_ttft_ms: 1800,
      min_success_rate: 0.95,
      pause_minutes: 15,
      min_success_samples: 8,
      min_ttft_samples: 6,
      condition: 'and'
    })

    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{
            id: 16,
            email: 'vip@x.com',
            deleted: false,
            allow: true,
            quality_max_p50_ttft_ms: 1500,
            quality_min_success_rate: 0.9
          }]
        })
      }
    })
    await wrapper.get('[data-testid="user-schedule-quality-edit-16"]').trigger('click')
    await wrapper.get('[data-testid="user-schedule-quality-apply-template"]').trigger('click')
    await flushPromises()

    expect(getQualityHardCloseSettings).toHaveBeenCalled()
    expect(wrapper.emitted('saveQuality')).toBeUndefined()
    expect(wrapper.get<HTMLInputElement>('[data-testid="user-schedule-quality-p50"]').element.value).toBe('1800')
    expect(wrapper.get<HTMLInputElement>('[data-testid="user-schedule-quality-window-n"]').element.value).toBe('8')
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.stability.applyTemplateSuccess')

    await wrapper.get('[data-testid="user-schedule-quality-save"]').trigger('click')
    expect(wrapper.emitted('saveQuality')?.[0]?.[0]).toEqual({
      user_id: 16,
      quality_max_p50_ttft_ms: 1800,
      quality_min_success_rate: 0.95,
      quality_min_success_samples: 8,
      quality_min_ttft_samples: 8,
      quality_condition: 'and'
    })
  })

  it('saves the current editor as the shared template without a user id', async () => {
    getQualityHardCloseSettings.mockReset()
    updateQualityHardCloseSettings.mockReset()
    showSuccess.mockReset()
    getQualityHardCloseSettings.mockResolvedValue({
      enabled: false,
      max_p50_ttft_ms: 1200,
      min_success_rate: 0.8,
      pause_minutes: 20,
      min_success_samples: 20,
      min_ttft_samples: 10,
      condition: 'or'
    })
    updateQualityHardCloseSettings.mockResolvedValue({
      enabled: false,
      max_p50_ttft_ms: 1600,
      min_success_rate: 0.91,
      pause_minutes: 20,
      min_success_samples: 20,
      min_ttft_samples: 10,
      condition: 'or'
    })

    const wrapper = mount(AccountUserScheduleCell, {
      props: {
        account: makeAccount({
          schedule_users: [{
            id: 16,
            email: 'vip@x.com',
            deleted: false,
            allow: true,
            quality_max_p50_ttft_ms: 1600,
            quality_min_success_rate: 0.91,
            quality_min_success_samples: 20,
            quality_min_ttft_samples: 10,
            quality_condition: 'or'
          }]
        })
      }
    })
    await wrapper.get('[data-testid="user-schedule-quality-edit-16"]').trigger('click')
    await wrapper.get('[data-testid="user-schedule-quality-save-template"]').trigger('click')
    await flushPromises()

    expect(updateQualityHardCloseSettings).toHaveBeenCalledWith({
      enabled: false,
      max_p50_ttft_ms: 1600,
      min_success_rate: 0.91,
      pause_minutes: 20,
      account_quality_window_n: 20,
      min_success_samples: 20,
      min_ttft_samples: 20,
      condition: 'or',
      schedule_use_failover_error_rate: false
    })
    expect(updateQualityHardCloseSettings.mock.calls[0][0]).not.toHaveProperty('user_id')
    expect(wrapper.emitted('saveQuality')).toBeUndefined()
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.stability.saveTemplateSuccess')
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
