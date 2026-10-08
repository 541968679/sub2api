import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import AccountStabilityUsagePanel from '../AccountStabilityUsagePanel.vue'

const { getUsageTrend, getStats, showError } = vi.hoisted(() => ({
  getUsageTrend: vi.fn(),
  getStats: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: { getUsageTrend },
    usage: { getStats }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const TokenUsageTrendStub = defineComponent({
  name: 'TokenUsageTrend',
  props: {
    trendData: { type: Array, default: () => [] },
    loading: { type: Boolean, default: false }
  },
  template: '<div data-test="token-trend">{{ trendData.length }}</div>'
})

const DateRangePickerStub = defineComponent({
  name: 'DateRangePicker',
  props: {
    startDate: { type: String, default: '' },
    endDate: { type: String, default: '' }
  },
  emits: ['update:startDate', 'update:endDate', 'change'],
  template: '<div data-test="usage-range">{{ startDate }}/{{ endDate }}</div>'
})

const SelectStub = defineComponent({
  name: 'Select',
  props: {
    modelValue: { type: String, default: '' }
  },
  emits: ['update:modelValue', 'change'],
  template: '<div data-test="usage-granularity">{{ modelValue }}</div>'
})

function mountPanel() {
  return mount(AccountStabilityUsagePanel, {
    props: { accountId: 12 },
    global: {
      stubs: {
        TokenUsageTrend: TokenUsageTrendStub,
        DateRangePicker: DateRangePickerStub,
        Select: SelectStub
      }
    }
  })
}

describe('AccountStabilityUsagePanel', () => {
  beforeEach(() => {
    getUsageTrend.mockReset()
    getStats.mockReset()
    showError.mockReset()
    getUsageTrend.mockResolvedValue({
      trend: [
        {
          date: '2026-10-08 11:00',
          requests: 4,
          input_tokens: 100,
          output_tokens: 20,
          cache_creation_tokens: 5,
          cache_read_tokens: 40,
          total_tokens: 165,
          cost: 1,
          actual_cost: 2
        }
      ],
      start_date: '2026-10-08',
      end_date: '2026-10-08',
      granularity: 'hour'
    })
    getStats.mockResolvedValue({
      total_requests: 4,
      total_input_tokens: 100,
      total_output_tokens: 20,
      total_cache_tokens: 45,
      total_tokens: 165,
      total_cost: 38.15,
      total_actual_cost: 4260,
      total_account_cost: 2.79,
      average_duration_ms: 0,
      total_cache_read_tokens: 40,
      total_cache_creation_tokens: 5,
      cache_hit_requests: 1,
      cache_read_rate: 0.2,
      cache_creation_rate: 0.1,
      request_hit_rate: 0.25
    })
  })

  it('loads this account’s hourly token trend and the three cost totals', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(getUsageTrend).toHaveBeenCalledTimes(1)
    expect(getUsageTrend).toHaveBeenCalledWith(
      expect.objectContaining({
        account_id: 12,
        granularity: 'hour',
        start_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
        end_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/)
      })
    )
    const trendArgs = getUsageTrend.mock.calls[0][0]
    expect(trendArgs.start_date).toBe(trendArgs.end_date)
    expect(getStats).toHaveBeenCalledWith({
      account_id: 12,
      start_date: trendArgs.start_date,
      end_date: trendArgs.end_date
    })
    expect(wrapper.get('[data-test="stability-usage-actual"]').text()).toContain('$4.26K')
    expect(wrapper.get('[data-test="stability-usage-standard"]').text()).toContain('$38.15')
    expect(wrapper.get('[data-test="stability-usage-account"]').text()).toContain('$2.79')
    expect(wrapper.get('[data-test="token-trend"]').text()).toBe('1')
    expect(wrapper.get('[data-test="usage-granularity"]').text()).toBe('hour')
  })

  it('clears the series and reports a load failure', async () => {
    getUsageTrend.mockRejectedValue(new Error('boom'))

    const wrapper = mountPanel()
    await flushPromises()

    expect(showError).toHaveBeenCalled()
    expect(wrapper.get('[data-test="token-trend"]').text()).toBe('0')
    expect(wrapper.get('[data-test="stability-usage-actual"]').text()).toContain('$0.0000')
  })
})
