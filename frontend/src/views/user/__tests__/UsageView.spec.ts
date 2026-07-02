import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import UsageView from '../UsageView.vue'

const { query, getStatsByDateRange, getDashboardTrend, list, showError } = vi.hoisted(() => ({
  query: vi.fn(),
  getStatsByDateRange: vi.fn(),
  getDashboardTrend: vi.fn(),
  list: vi.fn(),
  showError: vi.fn(),
}))

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'admin.usage.cacheReadCost': 'Cache Read Cost',
  'admin.usage.cacheCreationCost': 'Cache Creation Cost',
  'usage.tokenDetails': 'Token Details',
  'usage.totalTokens': 'Total Tokens',
  'admin.usage.inputTokens': 'Input Tokens',
  'admin.usage.outputTokens': 'Output Tokens',
  'admin.usage.cacheReadTokens': 'Cache Read Tokens',
  'admin.usage.cacheCreationTokens': 'Cache Creation Tokens',
  // 与真实 locale 一致：5m/1h 两个 key 文案相同，靠模板内的 5m/1h 徽章区分
  'admin.usage.cacheCreation5mTokens': 'Cache Write',
  'admin.usage.cacheCreation1hTokens': 'Cache Write',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierPriority': 'Fast',
  'usage.serviceTierFlex': 'Flex',
  'usage.serviceTierStandard': 'Standard',
  'usage.rate': 'Rate',
  'usage.original': 'Original',
  'usage.billed': 'Billed',
  'usage.allApiKeys': 'All API Keys',
  'usage.apiKeyFilter': 'API Key',
  'usage.model': 'Model',
  'usage.reasoningEffort': 'Reasoning Effort',
  'usage.type': 'Type',
  'usage.tokens': 'Tokens',
  'usage.cost': 'Cost',
  'usage.firstToken': 'First Token',
  'usage.duration': 'Duration',
  'usage.time': 'Time',
  'usage.userAgent': 'User Agent',
}

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStatsByDateRange,
    getDashboardTrend,
  },
  keysAPI: {
    list,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError }),
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

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="actions" /><slot name="filters" /><slot /></div>',
}

describe('user UsageView tooltip', () => {
  beforeEach(() => {
    query.mockReset()
    getStatsByDateRange.mockReset()
    getDashboardTrend.mockReset()
    list.mockReset()
    showError.mockReset()

    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 20,
      left: 20,
      right: 120,
      bottom: 40,
      width: 100,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)

    ;(globalThis as any).ResizeObserver = class {
      observe() {}
      disconnect() {}
    }
  })

  it('shows fast service tier and unit prices in user tooltip', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-1',
          actual_cost: 0.092883,
          total_cost: 0.092883,
          rate_multiplier: 1,
          service_tier: 'priority',
          input_cost: 0.020285,
          output_cost: 0.00303,
          cache_creation_cost: 0.12,
          cache_read_cost: 0.069568,
          input_tokens: 4057,
          output_tokens: 101,
          cache_creation_tokens: 12345,
          cache_read_tokens: 278272,
          cache_creation_5m_tokens: 11111,
          cache_creation_1h_tokens: 1234,
          cache_ttl_overridden: true,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    getDashboardTrend.mockResolvedValue({ trend: [], start_date: '2026-03-08', end_date: '2026-03-08', granularity: 'day' })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          UsageMetricTrendChart: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-1',
      actual_cost: 0.092883,
      total_cost: 0.092883,
      rate_multiplier: 1,
      service_tier: 'priority',
      input_cost: 0.020285,
      output_cost: 0.00303,
      cache_creation_cost: 0.12,
      cache_read_cost: 0.069568,
      input_tokens: 4057,
      output_tokens: 101,
      cache_creation_tokens: 12345,
      cache_read_tokens: 278272,
      display_input_price: 0.000005,
      display_output_price: 0.000030,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Service tier')
    expect(text).toContain('Fast')
    expect(text).toContain('Rate')
    expect(text).toContain('1.00x')
    expect(text).toContain('Billed')
    expect(text).toContain('$0.092883')
    expect(text).toContain('$5.0000 / 1M tokens')
    expect(text).toContain('$30.0000 / 1M tokens')
    expect(text).toContain('Cache Read Cost')
    expect(text).toContain('Cache Creation Cost')
    expect(text).toContain('$0.120000')
  })

  it('shows cache creation token details in the user token tooltip', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      avg_duration_ms: 0,
    })
    getDashboardTrend.mockResolvedValue({ trend: [], start_date: '2026-03-08', end_date: '2026-03-08', granularity: 'day' })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          UsageMetricTrendChart: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tokenTooltipData = {
      input_tokens: 10,
      output_tokens: 20,
      cache_read_tokens: 30,
      cache_creation_tokens: 40,
      cache_creation_5m_tokens: 25,
      cache_creation_1h_tokens: 15,
      cache_ttl_overridden: true,
    }
    setupState.tokenTooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Token Details')
    expect(text).toContain('Input Tokens')
    expect(text).toContain('Output Tokens')
    expect(text).toContain('Cache Read Tokens')
    expect(text).toContain('Total Tokens')
    // cache creation is now user-visible: 5m/1h breakdown rows plus the
    // creation tokens folded into the total (10 + 20 + 40 + 30 = 100)
    expect(text).toContain('Cache Write')
    expect(text).toContain('25')
    expect(text).toContain('15')
    expect(text).toContain('100')
    // the two breakdown rows share one label, so the 5m/1h badges must
    // carry the disambiguation
    expect(text).toContain('5m')
    expect(text).toContain('1h')
    // the admin-only cache TTL override badge stays hidden from users
    expect(text).not.toContain('R-')
  })

  it('uses backend display prices instead of deriving unit prices from rounded display tokens', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      avg_duration_ms: 0,
    })
    getDashboardTrend.mockResolvedValue({ trend: [], start_date: '2026-07-02', end_date: '2026-07-02', granularity: 'day' })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          UsageMetricTrendChart: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-fable-rounded',
      actual_cost: 0.00005,
      total_cost: 0.000025,
      rate_multiplier: 1.6,
      input_cost: 0.000025,
      output_cost: 0.0015,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 3,
      output_tokens: 30,
      cache_creation_tokens: 0,
      cache_read_tokens: 28041,
      display_input_price: 0.000010,
      display_output_price: 0.000050,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('$10.0000 / 1M tokens')
    expect(text).toContain('$50.0000 / 1M tokens')
    expect(text).not.toContain('$8.3333 / 1M tokens')
  })

  it('does not reverse-derive user model prices when backend unit prices are missing', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      avg_duration_ms: 0,
    })
    getDashboardTrend.mockResolvedValue({ trend: [], start_date: '2026-07-02', end_date: '2026-07-02', granularity: 'day' })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          UsageMetricTrendChart: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-no-unit-price',
      actual_cost: 0.00005,
      total_cost: 0.000025,
      rate_multiplier: 1.6,
      input_cost: 0.000025,
      output_cost: 0.0015,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 3,
      output_tokens: 30,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('- / 1M tokens')
    expect(text).not.toContain('$8.3333 / 1M tokens')
  })

})
