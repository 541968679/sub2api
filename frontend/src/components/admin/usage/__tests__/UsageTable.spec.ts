import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import UsageTable from '../UsageTable.vue'

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'admin.usage.cacheCreationCost': 'Cache Creation Cost',
  'admin.usage.cacheReadCost': 'Cache Read Cost',
  'admin.usage.cacheShare': '缓存占比：{share}',
  'admin.usage.cacheShareHint': 'cache share hint',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierPriority': 'Fast',
  'usage.serviceTierFlex': 'Flex',
  'usage.serviceTierStandard': 'Standard',
  'usage.rate': 'Rate',
  'usage.accountMultiplier': 'Account rate',
  'usage.original': 'Original',
  'usage.userBilled': 'User billed',
  'usage.userDisplayCost': 'User display',
  'usage.accountBilled': 'Account billed',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        const template = messages[key] ?? key
        if (!params) return template
        return Object.entries(params).reduce(
          (text, [name, value]) => text.replace(`{${name}}`, String(value)),
          template,
        )
      },
    }),
  }
})

const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.request_id">
        <slot name="cell-model" :row="row" :value="row.model" />
        <slot name="cell-tokens" :row="row" />
        <slot name="cell-display_tokens" :row="row" />
        <slot name="cell-cost" :row="row" />
      </div>
    </div>
  `,
}

describe('admin UsageTable tooltip', () => {
  beforeEach(() => {
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
  })

  it('shows service tier and billing breakdown in cost tooltip', async () => {
    const row = {
      request_id: 'req-admin-1',
      actual_cost: 0.092883,
      total_cost: 0.092883,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      service_tier: 'priority',
      input_cost: 0.020285,
      output_cost: 0.00303,
      cache_creation_cost: 0,
      cache_read_cost: 0.069568,
      input_tokens: 4057,
      output_tokens: 101,
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    // Cost tooltip is the last .group.relative (tokens/display_tokens tooltips come first)
    const groups = wrapper.findAll('.group.relative')
    await groups[groups.length - 1].trigger('mouseenter')
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Service tier')
    expect(text).toContain('Fast')
    expect(text).toContain('Rate')
    expect(text).toContain('1.00x')
    expect(text).toContain('Account rate')
    expect(text).toContain('User billed')
    expect(text).toContain('Account billed')
    expect(text).toContain('$0.092883')
    expect(text).toContain('$5.0000 / 1M tokens')
    expect(text).toContain('$30.0000 / 1M tokens')
    expect(text).toContain('$0.069568')
  })

  it('shows user display pricing fields for admin rows', async () => {
    const row = {
      request_id: 'req-admin-display-1',
      actual_cost: 0.0000476,
      total_cost: 0.0000476,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      input_cost: 0.0000408,
      output_cost: 0.0000068,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 408,
      output_tokens: 34,
      display_fields: {
        display_input_tokens: 4080,
        display_output_tokens: 340,
        display_cache_read_tokens: 0,
        display_input_cost: 0.0000408,
        display_output_cost: 0.0000068,
        display_cache_read_cost: 0,
        display_total_cost: 0.0000476,
      },
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    // Cost tooltip is the last .group.relative (tokens + display_tokens tooltips precede it)
    const groups = wrapper.findAll('.group.relative')
    await groups[groups.length - 1].trigger('mouseenter')
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('User display')
    expect(text).toContain('$0.1000 / 1M tokens')
    expect(text).toContain('$0.2000 / 1M tokens')
    expect(text).toContain('$0.0100 / 1M tokens')
    expect(text).toContain('$0.0200 / 1M tokens')
  })

  it('renders display tokens column from display_fields', () => {
    const row = {
      request_id: 'req-admin-display-tokens-1',
      actual_cost: 0.0000476,
      total_cost: 0.0000476,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      input_cost: 0.0000408,
      output_cost: 0.0000068,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 408,
      output_tokens: 34,
      cache_read_tokens: 0,
      cache_creation_tokens: 0,
      display_fields: {
        display_input_tokens: 4080,
        display_output_tokens: 340,
        display_cache_read_tokens: 1200,
        display_cache_creation_tokens: 500,
        display_input_cost: 0.0000408,
        display_output_cost: 0.0000068,
        display_cache_read_cost: 0,
        display_total_cost: 0.0000476,
      },
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('4,080')
    expect(text).toContain('340')
    // formatCacheTokens: 1200 -> "1.2K", 500 stays "500"
    expect(text).toContain('1.2K')
    expect(text).toContain('500')
  })

  it('shows cache share separately in token and TOKEN columns', () => {
    const row = {
      request_id: 'req-admin-cache-share-1',
      actual_cost: 0,
      total_cost: 0,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      // real: 20 / (40+10+20+30) = 20%
      input_tokens: 40,
      output_tokens: 10,
      cache_read_tokens: 20,
      cache_creation_tokens: 30,
      display_fields: {
        // display: 120 / (80+20+120+180) = 30%
        display_input_tokens: 80,
        display_output_tokens: 20,
        display_cache_read_tokens: 120,
        display_cache_creation_tokens: 180,
        display_input_cost: 0,
        display_output_cost: 0,
        display_cache_read_cost: 0,
        display_total_cost: 0,
      },
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('缓存占比：20.0%')
    expect(text).toContain('缓存占比：30.0%')
    expect(text).not.toContain('真实')
    expect(text).not.toContain('展示')
  })

  it('shows dash when display_fields is missing', () => {
    const row = {
      request_id: 'req-admin-no-display-1',
      actual_cost: 0,
      total_cost: 0,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 10,
      output_tokens: 5,
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    expect(wrapper.text()).toContain('-')
  })

  it('shows requested and upstream models separately for admin rows', () => {
    const row = {
      request_id: 'req-admin-model-1',
      model: 'claude-sonnet-4',
      upstream_model: 'claude-sonnet-4-20250514',
      actual_cost: 0,
      total_cost: 0,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 0,
      output_tokens: 0,
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('claude-sonnet-4')
    expect(text).toContain('claude-sonnet-4-20250514')
  })
})
