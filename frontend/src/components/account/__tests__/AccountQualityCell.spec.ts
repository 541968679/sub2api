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
    await button.trigger('click')
    expect(wrapper.emitted('click')).toHaveLength(1)
  })

  it('clickable keeps the existing tooltip on stats', () => {
    const wrapper = mount(AccountQualityCell, {
      props: { mode: 'success_rate', stats, clickable: true }
    })

    expect(wrapper.find('[title]').exists()).toBe(true)
    expect(wrapper.get('[title]').attributes('title')).toContain('admin.accounts.quality.tooltip')
  })
})
