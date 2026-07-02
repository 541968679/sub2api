import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import PricingView from '../PricingView.vue'

const { getUserPricingPage } = vi.hoisted(() => ({
  getUserPricingPage: vi.fn(),
}))

const messages: Record<string, string> = {
  'common.loading': 'Loading',
  'pricing.title': 'Pricing',
  'pricing.description': 'Model pricing',
  'pricing.introTitle': 'Intro',
  'pricing.educationTitle': 'Education',
  'pricing.tableTitle': 'Models',
  'pricing.cnyBanner': '1 USD = \u00a5{rate}',
  'pricing.emptyState': 'No models',
  'pricing.modelsSuffix': 'models',
  'pricing.columns.model': 'Model',
  'pricing.columns.billingMode': 'Billing mode',
  'pricing.columns.inputPrice': 'Input',
  'pricing.columns.outputPrice': 'Output',
  'pricing.columns.cacheReadPrice': 'Cache read',
  'pricing.billingMode.perToken': 'Token',
  'pricing.billingMode.perRequest': 'Per request',
  'pricing.billingMode.image': 'Image',
  'pricing.perRequestUnit': 'request',
  'pricing.unitHint': 'USD per million tokens',
}

vi.mock('@/api/pricingPage', () => ({
  pricingPageAPI: {
    getUserPricingPage,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: {
      payment_cny_per_usd: 7.2,
    },
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        (messages[key] ?? key).replace(/\{(\w+)\}/g, (_, name) => String(params?.[name] ?? '')),
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }

describe('user PricingView', () => {
  beforeEach(() => {
    getUserPricingPage.mockReset()
  })

  it('shows USD model prices without CNY conversion', async () => {
    getUserPricingPage.mockResolvedValue({
      intro: '',
      education: '',
      platforms: [
        {
          provider: 'anthropic',
          models: [
            {
              model: 'claude-fable-5',
              billing_mode: 'token',
              display_input_price: 0.0000083333,
              display_output_price: 0.000025,
              display_cache_read_price: 0.0000008,
              per_request_price: null,
            },
          ],
        },
      ],
    })

    const wrapper = mount(PricingView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
        },
      },
    })

    await flushPromises()

    const row = wrapper.findAll('tbody tr').find((item) => item.text().includes('claude-fable-5'))
    expect(row).toBeTruthy()
    expect(row!.text()).toContain('$8.3333')
    expect(row!.text()).toContain('$25')
    expect(row!.text()).toContain('$0.8')
    expect(row!.text()).not.toContain('\u00a5')
  })
})
