import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UsageStatsCards from '../UsageStatsCards.vue'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'

const messages: Record<string, string> = {
  'usage.totalRequests': 'Total Requests',
  'usage.inSelectedRange': 'in selected range',
  'usage.totalTokens': 'Total Tokens',
  'usage.in': 'In',
  'usage.out': 'Out',
  'usage.totalCost': 'Total Cost',
  'usage.accountCost': 'Account Cost',
  'usage.standardCost': 'Standard Cost',
  'usage.avgDuration': 'Avg Duration',
  'usage.cacheHitTitle': 'Cache Hit Rate',
  'usage.cacheCreationRate': 'Creation',
  'usage.cacheRequestHitRate': 'Req. Hit',
  'usage.cacheHitHint': 'hint',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const IconStub = { name: 'Icon', props: ['name', 'size'], template: '<i />' }

const mountCards = (stats: Partial<AdminUsageStatsResponse> | null) =>
  mount(UsageStatsCards, {
    props: { stats: stats as AdminUsageStatsResponse | null },
    global: { stubs: { Icon: IconStub } },
  })

describe('admin UsageStatsCards cache-hit card', () => {
  it('renders cache read rate as the main percentage and creation/request rates in the sub-line', () => {
    const stats: Partial<AdminUsageStatsResponse> = {
      total_requests: 100,
      total_input_tokens: 250,
      total_output_tokens: 100,
      total_tokens: 1000,
      total_cost: 1,
      total_actual_cost: 1,
      total_account_cost: 1,
      average_duration_ms: 100,
      total_cache_read_tokens: 700,
      total_cache_creation_tokens: 50,
      cache_hit_requests: 80,
      // read 700/(250+700+50)=0.70 ; creation 50/1000=0.05 ; request 80/100=0.80
      cache_read_rate: 0.7,
      cache_creation_rate: 0.05,
      request_hit_rate: 0.8,
    }
    const text = mountCards(stats).text()
    expect(text).toContain('Cache Hit Rate')
    expect(text).toContain('70.0%')
    expect(text).toContain('5.0%')
    expect(text).toContain('80.0%')
  })

  it('falls back to 0.0% when stats are null', () => {
    expect(mountCards(null).text()).toContain('0.0%')
  })
})
