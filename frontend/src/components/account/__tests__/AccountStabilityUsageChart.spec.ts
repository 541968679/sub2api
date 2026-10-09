import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import AccountStabilityUsageChart from '../AccountStabilityUsageChart.vue'
import type { TrendDataPoint } from '@/types'

vi.mock('vue-chartjs', () => ({
  Line: defineComponent({
    name: 'Line',
    props: {
      data: { type: Object, required: true },
      options: { type: Object, required: true }
    },
    template: '<div data-test="usage-line" />'
  })
}))

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  CategoryScale: {},
  LinearScale: {},
  PointElement: {},
  LineElement: {},
  Title: {},
  Tooltip: {},
  Legend: {},
  Filler: {}
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const points: TrendDataPoint[] = [
  {
    date: '2026-10-08 11:00',
    requests: 4,
    cache_hit_requests: 1,
    input_tokens: 100,
    output_tokens: 20,
    cache_creation_tokens: 5,
    cache_read_tokens: 40,
    total_tokens: 165,
    cost: 1,
    actual_cost: 2
  },
  {
    date: '2026-10-08 12:00',
    requests: 0,
    cache_hit_requests: 0,
    input_tokens: 0,
    output_tokens: 0,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    total_tokens: 0,
    cost: 0,
    actual_cost: 0
  }
]

describe('AccountStabilityUsageChart', () => {
  it('plots both cache rates as percentages and keeps cache creation as tokens', () => {
    const wrapper = mount(AccountStabilityUsageChart, { props: { trendData: points } })
    const datasets = wrapper.getComponent({ name: 'Line' }).props('data').datasets as Array<{
      label: string
      data: Array<number | null>
      yAxisID?: string
      spanGaps?: boolean
      rateKind?: string
      fill: boolean
    }>

    const requestRate = datasets.find((dataset) => dataset.rateKind === 'request')
    const tokenRatio = datasets.find((dataset) => dataset.rateKind === 'token')
    const creation = datasets.find((dataset) => dataset.label === 'admin.accounts.stability.usageCacheCreation')

    expect(requestRate?.yAxisID).toBe('yPercent')
    expect(requestRate?.spanGaps).toBe(true)
    expect(requestRate?.data[0]).toBe(25)
    expect(requestRate?.data[1]).toBeNull()
    expect(tokenRatio?.yAxisID).toBe('yPercent')
    expect(tokenRatio?.spanGaps).toBe(true)
    expect(tokenRatio?.data[0]).toBeCloseTo((40 / 165) * 100)
    expect(tokenRatio?.data[1]).toBeNull()
    expect(creation?.fill).toBe(false)
    expect(creation?.data).toEqual([5, 0])
    expect(datasets.some((dataset) => dataset.label === 'Cache Read')).toBe(false)
    expect(datasets.some((dataset) => Array.isArray(dataset.data) && dataset.data[0] === 40 && !dataset.rateKind)).toBe(
      false
    )

    const options = wrapper.getComponent({ name: 'Line' }).props('options') as {
      plugins: {
        tooltip: {
          callbacks: {
            label: (context: {
              dataset: { label?: string; rateKind?: 'request' | 'token' }
              dataIndex: number
              raw: unknown
            }) => string
          }
        }
      }
      scales: { yPercent: { min: number; max: number } }
    }
    expect(options.scales.yPercent).toMatchObject({ min: 0, max: 100 })
    expect(
      options.plugins.tooltip.callbacks.label({
        dataset: { label: '缓存读取率', rateKind: 'request' },
        dataIndex: 0,
        raw: 25
      })
    ).toBe('缓存读取率: 25.0% (1 / 4)')
    expect(
      options.plugins.tooltip.callbacks.label({
        dataset: { label: '缓存读取比例', rateKind: 'token' },
        dataIndex: 0,
        raw: (40 / 165) * 100
      })
    ).toBe('缓存读取比例: 24.2% (40 / 165)')
    expect(
      options.plugins.tooltip.callbacks.label({
        dataset: { label: '缓存读取率', rateKind: 'request' },
        dataIndex: 1,
        raw: null
      })
    ).toBe('缓存读取率: — (—)')
  })
})