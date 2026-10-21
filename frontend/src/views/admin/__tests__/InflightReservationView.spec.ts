import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import InflightReservationView from '../InflightReservationView.vue'
import type { InflightReservationSettings } from '@/api/admin/inflightReservation'

const { getSettings, updateSettings, showError, showSuccess } = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    inflightReservation: {
      get: getSettings,
      update: updateSettings,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_err: unknown, fallback: string) => fallback,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (!params) return key
        return `${key} ${Object.values(params).join(' ')}`
      },
    }),
  }
})

const loaded = (): InflightReservationSettings => ({
  enabled: true,
  ttl_seconds: 900,
  bytes_per_token: 4,
  default_max_tokens: 8192,
  max_output_tokens: 128000,
  max_input_tokens: 200000,
  input_pricing: 'full',
  full_price_input_tokens: 4096,
  max_reservation_amount: 0,
  fail_closed_on_unpriced: false,
  source: 'config',
  simple_mode: false,
})

describe('admin InflightReservationView', () => {
  beforeEach(() => {
    getSettings.mockReset()
    updateSettings.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    getSettings.mockResolvedValue(loaded())
    updateSettings.mockImplementation(async (payload: InflightReservationSettings) => ({
      ...payload,
      source: 'settings',
    }))
  })

  it('saves the hold cap and cache-tail rule from the form', async () => {
    const wrapper = mount(InflightReservationView)
    await flushPromises()

    expect(wrapper.get('[data-test="inflight-source"]').text()).toContain('sourceConfig')
    expect(wrapper.get('[data-test="inflight-preview-amount"]').text()).toContain('0.9433')

    await wrapper.get('[data-test="inflight-pricing-cache-tail"]').setValue(true)
    await wrapper.get('[data-test="inflight-default-max-tokens"]').setValue('1024')
    await wrapper.get('[data-test="inflight-max-reservation"]').setValue('0.3')
    await wrapper.get('[data-test="inflight-save"]').trigger('click')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
      input_pricing: 'cache_tail',
      default_max_tokens: 1024,
      max_reservation_amount: 0.3,
      enabled: true,
    }))
    expect(showSuccess).toHaveBeenCalled()
    expect(wrapper.get('[data-test="inflight-preview-amount"]').text()).toContain('0.1367')
    expect(wrapper.get('[data-test="inflight-source"]').text()).toContain('sourceSettings')
  })
})
