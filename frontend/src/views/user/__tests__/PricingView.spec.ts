import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

import PricingView from '../PricingView.vue'

const getUserPricingPage = vi.hoisted(() => vi.fn())
const getAvailable = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: { payment_cny_per_usd: 0 }
  })
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: defineComponent({
    name: 'AppLayout',
    setup(_, { slots }) {
      return () => h('div', { 'data-test': 'app-layout' }, slots.default?.())
    }
  })
}))

vi.mock('@/components/common/SearchInput.vue', () => ({
  default: defineComponent({
    name: 'SearchInput',
    props: {
      modelValue: { type: String, default: '' },
      placeholder: { type: String, default: '' }
    },
    emits: ['update:modelValue', 'search'],
    setup(props, { emit }) {
      return () =>
        h('input', {
          'data-test': 'pricing-model-search-input',
          value: props.modelValue,
          placeholder: props.placeholder,
          onInput: (event: Event) =>
            emit('update:modelValue', (event.target as HTMLInputElement).value)
        })
    }
  })
}))

vi.mock('@/api/pricingPage', () => ({
  pricingPageAPI: {
    getUserPricingPage
  }
}))

vi.mock('@/api/groups', () => ({
  userGroupsAPI: {
    getAvailable
  }
}))

vi.mock('marked', () => ({
  marked: {
    setOptions: vi.fn(),
    parse: (text: string) => `<p>${text}</p>`
  }
}))

vi.mock('dompurify', () => ({
  default: {
    sanitize: (html: string) => html
  }
}))

const pricingFixture = {
  intro: 'intro body',
  education: 'education body',
  platforms: [
    {
      provider: 'anthropic',
      models: [
        {
          model: 'claude-sonnet-4',
          billing_mode: 'per_token',
          display_input_price: 0.000003,
          display_output_price: 0.000015,
          display_cache_read_price: 0.0000003,
          per_request_price: null
        }
      ]
    },
    {
      provider: 'openai',
      models: [
        {
          model: 'gpt-5',
          billing_mode: 'per_token',
          display_input_price: 0.000001,
          display_output_price: 0.000002,
          display_cache_read_price: null,
          per_request_price: null
        },
        {
          model: 'glm-5.3',
          billing_mode: 'per_token',
          display_input_price: 0.000002,
          display_output_price: 0.00003,
          display_cache_read_price: 0.0000004,
          per_request_price: null
        }
      ]
    }
  ]
}

const groupsFixture = [
  {
    id: 11,
    name: 'Claude Standard',
    description: null,
    platform: 'anthropic',
    rate_multiplier: 1.25,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'standard',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    allow_image_generation: false,
    allow_batch_image_generation: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    batch_image_discount_multiplier: 1,
    batch_image_hold_multiplier: 1,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    video_price_480p: null,
    video_price_720p: null,
    video_price_1080p: null,
    web_search_price_per_call: null,
    claude_code_only: false
  }
]

describe('PricingView', () => {
  beforeEach(() => {
    getUserPricingPage.mockReset()
    getAvailable.mockReset()
    getUserPricingPage.mockResolvedValue(pricingFixture)
    getAvailable.mockResolvedValue(groupsFixture)
  })

  it('renders billing explainer and dual tables with platform tabs', async () => {
    const wrapper = mount(PricingView)
    await flushPromises()

    expect(wrapper.text()).toContain('pricing.billingExplainerTitle')
    expect(wrapper.text()).toContain('intro body')
    expect(wrapper.text()).not.toContain('education body')
    expect(wrapper.text()).toContain('pricing.modelTableTitle')
    expect(wrapper.text()).toContain('pricing.groupTableTitle')
    expect(wrapper.text()).not.toContain('pricing.groupRateHint')

    expect(wrapper.find('[data-test="pricing-platform-tab-anthropic"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="pricing-platform-tab-openai"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('claude-sonnet-4')
    expect(wrapper.text()).not.toContain('gpt-5')

    await wrapper.get('[data-test="pricing-platform-tab-openai"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('gpt-5')
    expect(wrapper.text()).not.toContain('claude-sonnet-4')
    expect(wrapper.text()).not.toContain('glm-5.3')

    expect(wrapper.find('[data-test="pricing-platform-tab-domestic"]').exists()).toBe(true)
    await wrapper.get('[data-test="pricing-platform-tab-domestic"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('glm-5.3')
    expect(wrapper.text()).not.toContain('gpt-5')

    expect(wrapper.find('[data-test="pricing-group-row-11"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Claude Standard')
    expect(wrapper.text()).toContain('×1.25')
    expect(getAvailable).toHaveBeenCalledTimes(1)
  })

  it('keeps model table when groups fail', async () => {
    getAvailable.mockRejectedValue(new Error('groups down'))

    const wrapper = mount(PricingView)
    await flushPromises()

    expect(wrapper.text()).toContain('claude-sonnet-4')
    expect(wrapper.find('[data-test="pricing-groups-error"]').text()).toContain('groups down')
    expect(wrapper.find('[data-test="pricing-group-row-11"]').exists()).toBe(false)
  })

  it('shows groups empty state without calling rates API', async () => {
    getAvailable.mockResolvedValue([])

    const wrapper = mount(PricingView)
    await flushPromises()

    expect(wrapper.find('[data-test="pricing-groups-empty"]').exists()).toBe(true)
    expect(getAvailable).toHaveBeenCalledTimes(1)
  })

  it('searches model names and jumps to the matching tab', async () => {
    const wrapper = mount(PricingView)
    await flushPromises()

    await wrapper.get('[data-test="pricing-model-search-input"]').setValue('glm-5.3')
    await flushPromises()

    expect(wrapper.text()).toContain('glm-5.3')
    expect(wrapper.text()).not.toContain('gpt-5')
    expect(wrapper.text()).not.toContain('claude-sonnet-4')
    expect(wrapper.get('[data-test="pricing-platform-tab-domestic"]').attributes('aria-selected')).toBe('true')
  })

  it('searches displayed unit prices', async () => {
    const wrapper = mount(PricingView)
    await flushPromises()

    await wrapper.get('[data-test="pricing-model-search-input"]').setValue('30.00')
    await flushPromises()

    expect(wrapper.text()).toContain('glm-5.3')
    expect(wrapper.text()).not.toContain('gpt-5')
  })

  it('shows search empty state when nothing matches', async () => {
    const wrapper = mount(PricingView)
    await flushPromises()

    await wrapper.get('[data-test="pricing-model-search-input"]').setValue('no-such-model')
    await flushPromises()

    expect(wrapper.find('[data-test="pricing-search-empty"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('pricing.searchEmpty')
  })
})
