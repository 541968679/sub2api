import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserTokenRanking from '../UserTokenRanking.vue'

const getUserBreakdown = vi.fn()

vi.mock('@/api/admin/dashboard', () => ({
  getUserBreakdown: (...args: unknown[]) => getUserBreakdown(...args),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('UserTokenRanking', () => {
  beforeEach(() => {
    getUserBreakdown.mockReset()
    getUserBreakdown.mockResolvedValue({
      users: [{
        user_id: 7,
        email: 'u7@test.com',
        requests: 3,
        input_tokens: 100,
        output_tokens: 20,
        cache_creation_tokens: 11,
        cache_read_tokens: 13,
        total_tokens: 144,
        cost: 1.25,
        actual_cost: 0.75,
        account_cost: 0.5,
      }],
    })
  })

  it('loads with shared requested-model filters and emits a drilldown target', async () => {
    const wrapper = mount(UserTokenRanking, {
      props: {
        startDate: '2026-07-01',
        endDate: '2026-07-08',
        filters: { group_id: 3, model_source: 'requested' },
        model: 'claude-fable-5',
      },
      global: { stubs: { Select: true, LoadingSpinner: true } },
    })
    await flushPromises()

    expect(getUserBreakdown).toHaveBeenCalledWith(expect.objectContaining({
      group_id: 3,
      model_source: 'requested',
      model: 'claude-fable-5',
      sort_by: 'total_tokens',
      limit: 50,
    }))
    await wrapper.find('tbody tr').trigger('click')
    expect(wrapper.emitted('select-user')?.[0]).toEqual([7, 'u7@test.com'])
    expect(wrapper.text()).toContain('13')
  })
})
