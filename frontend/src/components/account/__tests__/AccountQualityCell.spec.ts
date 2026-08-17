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

  it('combined mode stacks p50 and success rate', () => {
    const wrapper = mount(AccountQualityCell, {
      props: { mode: 'combined', stats, clickable: true }
    })

    expect(wrapper.text()).toContain('300ms')
    expect(wrapper.text()).toContain('91.0%')
    expect(wrapper.get('[title]').attributes('title')).toContain('admin.accounts.quality.ttftTooltip')
    expect(wrapper.get('[title]').attributes('title')).toContain('admin.accounts.quality.tooltip')
    expect(wrapper.get('[title]').attributes('title')).not.toContain('admin.accounts.quality.bridgeTooltip')
    expect(wrapper.text()).not.toContain('admin.accounts.quality.bridgeShort')
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

  it('hides the rate when samples are below minSamples', () => {
    const wrapper = mount(AccountQualityCell, {
      props: { mode: 'success_rate', stats, minSamples: 20 }
    })

    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).not.toContain('91.0%')
  })
})
