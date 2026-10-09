import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import TokenUsageTrend from '../TokenUsageTrend.vue'
import type { TrendDataPoint } from '@/types'

vi.mock('vue-chartjs', () => ({
  Line: defineComponent({
    name: 'Line',
    props: {
      data: { type: Object, required: true },
      options: { type: Object, required: true }
    },
    template: '<div data-test="token-line" />'
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
    requests: 2,
    input_tokens: 10,
    output_tokens: 4,
    cache_creation_tokens: 30,
    cache_read_tokens: 400,
    total_tokens: 444,
    cost: 1,
    actual_cost: 1
  }
]

describe('TokenUsageTrend', () => {
  it('draws Cache Creation as an unfilled line above the Cache Read area', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: { trendData: points }
    })
    const datasets = wrapper.getComponent({ name: 'Line' }).props('data').datasets as Array<{
      label: string
      fill: boolean
      order: number
      data: number[]
    }>
    const creation = datasets.find((dataset) => dataset.label === 'Cache Creation')
    const read = datasets.find((dataset) => dataset.label === 'Cache Read')
    expect(creation).toBeTruthy()
    expect(creation?.fill).toBe(false)
    expect(creation?.data).toEqual([30])
    expect(read?.fill).toBe(true)
    expect(creation!.order).toBeLessThan(read!.order)
  })
})
