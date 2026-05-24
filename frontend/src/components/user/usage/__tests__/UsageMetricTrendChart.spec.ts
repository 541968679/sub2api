import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UsageMetricTrendChart from '../UsageMetricTrendChart.vue'

const messages: Record<string, string> = {
  'usage.trend.title': 'Usage Trend',
  'usage.trend.subtitle': 'Up to 4 metrics',
  'usage.trend.noData': 'No trend data',
  'usage.trend.maxMetricsHint': 'Up to 4 metrics can be shown at once',
  'usage.trend.metrics.actualCost': 'Total Cost',
  'usage.trend.metrics.totalTokens': 'Total Tokens',
  'usage.trend.metrics.requests': 'Requests',
  'usage.trend.metrics.inputTokens': 'Input Tokens',
  'usage.trend.metrics.outputTokens': 'Output Tokens',
  'usage.trend.metrics.cacheWriteTokens': 'Cache Write',
  'usage.trend.metrics.cacheReadTokens': 'Cache Read',
  'usage.trend.metrics.standardCost': 'Standard Cost',
  'common.loading': 'Loading',
}

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template: '<div data-test="line-chart">{{ data.datasets.map((dataset) => dataset.label).join("|") }}</div>',
  },
}))

vi.mock('chart.js', () => ({
  Chart: {
    register: vi.fn(),
  },
  CategoryScale: {},
  LinearScale: {},
  PointElement: {},
  LineElement: {},
  Tooltip: {},
  Legend: {},
  Filler: {},
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const trendData = [
  {
    date: '2026-05-24',
    requests: 12,
    input_tokens: 100,
    output_tokens: 200,
    cache_creation_tokens: 30,
    cache_read_tokens: 40,
    total_tokens: 370,
    cost: 0.4,
    actual_cost: 0.32,
  },
]

describe('UsageMetricTrendChart', () => {
  it('always includes total cost and total tokens', () => {
    const wrapper = mount(UsageMetricTrendChart, {
      props: {
        trendData,
      },
      global: {
        stubs: {
          Icon: true,
          LoadingSpinner: true,
        },
      },
    })

    const chartText = wrapper.get('[data-test="line-chart"]').text()
    expect(chartText).toContain('Total Cost')
    expect(chartText).toContain('Total Tokens')
    expect(chartText).toContain('Requests')
  })

  it('limits optional metric selection to two extra series', async () => {
    const wrapper = mount(UsageMetricTrendChart, {
      props: {
        trendData,
      },
      global: {
        stubs: {
          Icon: true,
          LoadingSpinner: true,
        },
      },
    })

    await wrapper.get('button:nth-of-type(2)').trigger('click')

    const disabledButtons = wrapper.findAll('button:disabled')
    expect(disabledButtons.length).toBeGreaterThan(0)
    expect(wrapper.get('[data-test="line-chart"]').text().split('|')).toHaveLength(4)
    expect(wrapper.text()).toContain('Up to 4 metrics can be shown at once')
  })
})
