import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SmartSchedulePnlCell from '../SmartSchedulePnlCell.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => {
      if (params) return `${key}:${Object.values(params).join(',')}`
      return key
    }
  })
}))

vi.mock('@/components/account/UsageProgressBar.vue', () => ({
  default: {
    name: 'UsageProgressBar',
    props: ['label', 'utilization', 'resetsAt', 'color'],
    template: '<div class="usage-progress-bar">{{ label }} {{ Math.round(Number(utilization) || 0) }}%</div>'
  }
}))

function accountWithBalance(overrides: Partial<Account> = {}): Account {
  return {
    id: 11,
    name: 'acc-11',
    platform: 'openai',
    type: 'apikey',
    extra: {
      upstream_balance_usd: 80,
      burn_samples: [
        { t: '2026-08-17T10:00:00.000Z', v: 100, kind: 'balance_usd' },
        { t: '2026-08-17T11:00:00.000Z', v: 90, kind: 'balance_usd' },
        { t: '2026-08-17T12:00:00.000Z', v: 80, kind: 'balance_usd' }
      ]
    },
    ...overrides
  } as Account
}

describe('SmartSchedulePnlCell', () => {
  it('shows today profit, balance, and no 7-day window', () => {
    const wrapper = mount(SmartSchedulePnlCell, {
      props: {
        account: accountWithBalance(),
        summary: {
          today: { revenue: 1.2, cost: 0.3, profit: 0.9, margin: 0.75 },
          seven_day: { revenue: 9, cost: 3, profit: 6, margin: 0.66 }
        }
      }
    })
    expect(wrapper.get('[data-testid="smart-schedule-pnl-balance"]').text()).toContain('$80.00')
    expect(wrapper.get('[data-testid="smart-schedule-pnl-margin"]').text()).toBe('75.0%')
    expect(wrapper.text()).toContain('+$0.90')
    expect(wrapper.text()).not.toContain('$9.00')
    expect(wrapper.text()).not.toContain('admin.users.schedulePnl.sevenDay')
    expect(wrapper.find('[data-testid="smart-schedule-pnl-oauth-7d"]').exists()).toBe(false)
  })

  it('replaces the balance row with a 7-day quota bar for oauth accounts', () => {
    const wrapper = mount(SmartSchedulePnlCell, {
      props: {
        account: accountWithBalance({
          type: 'oauth',
          platform: 'anthropic',
          extra: {
            passive_usage_7d_utilization: 0.42,
            passive_usage_7d_reset: Math.floor(Date.now() / 1000) + 3600,
            upstream_balance_usd: 80
          }
        }),
        summary: {
          today: { revenue: 1.2, cost: 0.3, profit: 0.9, margin: 0.75 },
          seven_day: { revenue: 9, cost: 3, profit: 6, margin: 0.66 }
        }
      }
    })
    expect(wrapper.find('[data-testid="smart-schedule-pnl-balance"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="smart-schedule-pnl-oauth-7d"]').text()).toContain('42%')
    expect(wrapper.get('[data-testid="smart-schedule-pnl-oauth-7d"]').text()).toContain('7d')
    expect(wrapper.text()).toContain('+$0.90')
    expect(wrapper.text()).not.toContain('$80.00')
    expect(wrapper.text()).not.toContain('admin.users.schedulePnl.sevenDay')
    expect(wrapper.find('[data-testid="smart-schedule-pnl-burn"]').exists()).toBe(false)
  })

  it('shows a dash when an oauth account has no 7-day snapshot', () => {
    const wrapper = mount(SmartSchedulePnlCell, {
      props: {
        account: accountWithBalance({
          type: 'oauth',
          platform: 'openai',
          extra: { upstream_balance_usd: 80 }
        }),
        summary: {
          today: { revenue: 1.2, cost: 0.3, profit: 0.9, margin: 0.75 },
          seven_day: null
        }
      }
    })
    expect(wrapper.find('[data-testid="smart-schedule-pnl-balance"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="smart-schedule-pnl-oauth-7d"]').text()).toBe('—')
  })

  it('shows a compact burn alignment cue from cached extra samples', () => {
    const noon = new Date('2026-08-17T12:00:00')
    vi.useFakeTimers()
    vi.setSystemTime(noon)
    const wrapper = mount(SmartSchedulePnlCell, {
      props: {
        account: accountWithBalance(),
        summary: {
          today: { revenue: 1.2, cost: 0.3, profit: 0.9, margin: 0.75 },
          seven_day: null
        },
        todayStats: { requests: 4, tokens: 100, cost: 120 }
      }
    })
    const burn = wrapper.get('[data-testid="smart-schedule-pnl-burn"]')
    expect(burn.attributes('data-burn-status')).toBe('match')
    expect(burn.text()).toContain('admin.users.schedulePnl.burnMatch')
    vi.useRealTimers()
  })
})
