import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpenAIQuotaResetCell from '../OpenAIQuotaResetCell.vue'
import type { Account } from '@/types'
import { queryOpenAIQuota, resetOpenAIQuota } from '@/api/admin/accounts'

vi.mock('@/api/admin/accounts', () => ({
  queryOpenAIQuota: vi.fn(),
  resetOpenAIQuota: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

function makeAccount(overrides: Partial<Account> = {}): Account {
  return {
    id: 1,
    name: 'OpenAI OAuth',
    platform: 'openai',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-07-11T00:00:00Z',
    updated_at: '2026-07-11T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  }
}

describe('OpenAIQuotaResetCell', () => {
  beforeEach(() => {
    vi.mocked(queryOpenAIQuota).mockReset()
    vi.mocked(resetOpenAIQuota).mockReset()
  })

  it('is hidden for Grok accounts so the independent Grok quota probe remains authoritative', () => {
    const wrapper = mount(OpenAIQuotaResetCell, {
      props: { account: makeAccount({ platform: 'grok' }) }
    })

    expect(wrapper.find('[data-testid="openai-quota-actions"]').exists()).toBe(false)
  })

  it('requires a confirmed action before consuming a reset credit', async () => {
    vi.mocked(queryOpenAIQuota)
      .mockResolvedValueOnce({
        rate_limit_reset_credits: {
          available_count: 2,
          credits: [{ expires_at: '2026-07-12T08:00:00Z' }]
        },
        fetched_at: 1
      })
      .mockResolvedValueOnce({
        rate_limit_reset_credits: { available_count: 1 },
        fetched_at: 2
      })
    vi.mocked(resetOpenAIQuota).mockResolvedValue({ code: 'ok', windows_reset: 1 })

    const wrapper = mount(OpenAIQuotaResetCell, {
      props: { account: makeAccount() },
      global: {
        stubs: {
          ConfirmDialog: {
            props: ['show'],
            emits: ['confirm', 'cancel'],
            template: '<button v-if="show" data-testid="confirm-reset" @click="$emit(\'confirm\')">confirm</button>'
          }
        }
      }
    })

    await wrapper.get('[data-testid="query-openai-quota"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('2')
    expect(wrapper.find('[title*="2026"]').exists()).toBe(true)

    await wrapper.get('[data-testid="reset-openai-quota"]').trigger('click')
    expect(resetOpenAIQuota).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="confirm-reset"]').trigger('click')
    await flushPromises()

    expect(resetOpenAIQuota).toHaveBeenCalledTimes(1)
    expect(resetOpenAIQuota).toHaveBeenCalledWith(1)
    expect(queryOpenAIQuota).toHaveBeenCalledTimes(2)
  })
})
