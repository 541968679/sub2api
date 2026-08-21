import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountQualityCell from '../AccountQualityCell.vue'
import type { AccountQualityStats } from '@/api/admin/accounts'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const stats: AccountQualityStats = {
  window_seconds: 900,
  success_count: 10,
  error_count: 1,
  success_rate: 0.91,
  avg_ttft_ms: 400,
  p50_ttft_ms: 300,
  p95_ttft_ms: 900,
  max_ttft_ms: 1200,
  ttft_samples: 10
}

describe('AccountQualityCell', () => {
  it('default is not clickable and does not emit click', async () => {
    const wrapper = mount(AccountQualityCell, {
      props: { mode: 'ttft', stats }
    })

    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.find('[data-test="account-quality-cell"]').exists()).toBe(true)
    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toBeFalsy()
  })

  it('subject=user uses user-scoped test id and aria copy', () => {
    const wrapper = mount(AccountQualityCell, {
      props: { mode: 'combined', stats, clickable: true, subject: 'user', windowN: 20 }
    })

    expect(wrapper.get('[data-test="user-quality-cell-button"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="account-quality-cell-button"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="user-quality-cell-button"]').attributes('aria-label')).toBe(
      'admin.users.quality.openAria'
    )
    expect(wrapper.get('[data-test="account-quality-window-counts"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="account-quality-failover-rate"]').exists()).toBe(true)
  })

  it('clickable emits click even when the cell is empty', async () => {
    const wrapper = mount(AccountQualityCell, {
      props: { mode: 'ttft', clickable: true }
    })

    const button = wrapper.get('[data-test="account-quality-cell-button"]')
    expect(button.element.tagName).toBe('BUTTON')
    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).toContain('admin.accounts.stability.openShort')
    await button.trigger('click')
    expect(wrapper.emitted('click')).toHaveLength(1)
  })

  it('combined mode stacks p50, schedule success rate, and failover success rate', () => {
    const wrapper = mount(AccountQualityCell, {
      props: {
        mode: 'combined',
        stats: {
          ...stats,
          terminal_error_count: 1,
          terminal_error_rate: 1 / 11,
          failover_error_count: 20,
          failover_error_rate: 20 / 30
        },
        clickable: true
      }
    })

    expect(wrapper.text()).toContain('300ms')
    expect(wrapper.text()).toContain('91.0%')
    expect(wrapper.text()).toContain('33.3%')
    expect(wrapper.text()).not.toContain('66.7%')
    expect(wrapper.text()).toContain('admin.accounts.quality.successShort')
    expect(wrapper.text()).toContain('admin.accounts.quality.failoverShort')
    expect(wrapper.get('[data-test="account-quality-window-counts"]').text()).toContain(
      'admin.accounts.quality.windowCounts'
    )
    const failoverRow = wrapper.get('[data-test="account-quality-failover-rate"]')
    expect(failoverRow.text()).toContain('33.3%')
    expect(failoverRow.classes().join(' ')).toContain('text-red-600')
    expect(wrapper.get('[title]').attributes('title')).toContain('admin.accounts.quality.ttftTooltip')
    expect(wrapper.get('[title]').attributes('title')).toContain('admin.accounts.quality.tooltip')
    expect(wrapper.get('[title]').attributes('title')).toContain('admin.accounts.quality.failoverTooltip')
    expect(wrapper.get('[title]').attributes('title')).not.toContain('admin.accounts.quality.bridgeTooltip')
    expect(wrapper.text()).not.toContain('admin.accounts.quality.bridgeShort')
  })

  it('colors failover success rate like schedule success, not like an error-rate scale', () => {
    const amber = mount(AccountQualityCell, {
      props: {
        mode: 'combined',
        stats: {
          ...stats,
          success_rate: 0.96,
          failover_error_count: 8,
          failover_error_rate: 0.08
        }
      }
    })
    expect(amber.get('[data-test="account-quality-failover-rate"]').text()).toContain('92.0%')
    expect(amber.get('[data-test="account-quality-failover-rate"]').classes().join(' ')).toContain('text-amber-600')

    const good = mount(AccountQualityCell, {
      props: {
        mode: 'combined',
        stats: {
          ...stats,
          success_rate: 0.99,
          failover_error_count: 1,
          failover_error_rate: 0.02
        }
      }
    })
    expect(good.get('[data-test="account-quality-failover-rate"]').text()).toContain('98.0%')
    expect(good.get('[data-test="account-quality-failover-rate"]').classes().join(' ')).toContain('text-emerald-600')
  })

  it('combined mode does not render an in-cell bridge error rate', () => {
    const wrapper = mount(AccountQualityCell, {
      props: {
        mode: 'combined',
        stats: {
          ...stats,
          bridge_success_count: 4,
          bridge_error_count: 6,
          bridge_error_rate: 0.6
        }
      }
    })

    expect(wrapper.text()).toContain('300ms')
    expect(wrapper.text()).toContain('91.0%')
    expect(wrapper.text()).not.toContain('admin.accounts.quality.bridgeShort')
    expect(wrapper.text()).not.toContain('60.0%')
    expect(wrapper.get('[title]').attributes('title')).not.toContain('admin.accounts.quality.bridgeTooltip')
    expect(wrapper.get('[data-test="account-quality-failover-rate"]').text()).toContain('—')
  })

  it('clickable keeps the existing tooltip on stats', () => {
    const wrapper = mount(AccountQualityCell, {
      props: { mode: 'success_rate', stats, clickable: true }
    })

    expect(wrapper.find('[title]').exists()).toBe(true)
    expect(wrapper.get('[title]').attributes('title')).toContain('admin.accounts.quality.tooltip')
  })

  it('does not render 0% for an empty window even when JSON emits 0', () => {
    const wrapper = mount(AccountQualityCell, {
      props: {
        mode: 'combined',
        stats: {
          window_seconds: 900,
          success_count: 0,
          error_count: 0,
          success_rate: 0,
          error_rate: 0,
          terminal_error_count: 0,
          terminal_error_rate: 0,
          failover_error_count: 0,
          failover_error_rate: 0,
          bridge_success_count: 0,
          bridge_error_count: 0,
          bridge_error_rate: 0,
          avg_ttft_ms: null,
          p50_ttft_ms: null,
          p95_ttft_ms: null,
          max_ttft_ms: null,
          ttft_samples: 0
        }
      }
    })

    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).not.toContain('0.0%')
    expect(wrapper.text()).not.toContain('100.0%')
    expect(wrapper.find('[data-test="account-quality-failover-rate"]').exists()).toBe(false)
  })

  it('does not render 0% for error-only windows with no completed usage', () => {
    const wrapper = mount(AccountQualityCell, {
      props: {
        mode: 'success_rate',
        stats: {
          window_seconds: 900,
          success_count: 0,
          error_count: 33,
          success_rate: 0,
          error_rate: 1,
          avg_ttft_ms: null,
          p50_ttft_ms: null,
          p95_ttft_ms: null,
          max_ttft_ms: null,
          ttft_samples: 0
        }
      }
    })

    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).not.toContain('0.0%')
  })

  it('success_rate mode keeps schedule success only and does not print failover', () => {
    const wrapper = mount(AccountQualityCell, {
      props: {
        mode: 'success_rate',
        stats: {
          ...stats,
          failover_error_count: 20,
          failover_error_rate: 20 / 30
        }
      }
    })

    expect(wrapper.text()).toContain('91.0%')
    expect(wrapper.text()).not.toContain('33.3%')
    expect(wrapper.text()).not.toContain('66.7%')
    expect(wrapper.text()).not.toContain('admin.accounts.quality.failoverShort')
    expect(wrapper.find('[data-test="account-quality-failover-rate"]').exists()).toBe(false)
  })

  it('hides the rate when samples are below minSamples', () => {
    const wrapper = mount(AccountQualityCell, {
      props: { mode: 'success_rate', stats, minSamples: 20 }
    })

    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).not.toContain('91.0%')
  })

  it('shows last-N counts from stats.window_n', () => {
    const wrapper = mount(AccountQualityCell, {
      props: {
        mode: 'combined',
        stats: { ...stats, window_n: 20, success_count: 10, error_count: 1, ttft_samples: 10 }
      }
    })

    expect(wrapper.get('[data-test="account-quality-window-counts"]').exists()).toBe(true)
    expect(wrapper.get('[title]').attributes('title')).toContain('admin.accounts.quality.ttftTooltip')
  })
})
