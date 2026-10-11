import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CardShopRedeemPanel from '../CardShopRedeemPanel.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (
      params ? `${key} ${JSON.stringify(params)}` : key
    ),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { username: 'ada', balance: 12.5 },
  }),
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    activeSubscriptions: [],
  }),
}))

vi.mock('@/api/redeem', () => ({
  redeemAPI: { redeem: vi.fn() },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

const notice = `线上支付渠道暂不可用
请直接从
https://pay.ldxp.cn/shop/8AJ8M782
购买兑换码进行兑换即可`

describe('CardShopRedeemPanel', () => {
  it('turns the notice url into the card-shop button and keeps redeem on the page', () => {
    const wrapper = mount(CardShopRedeemPanel, {
      props: { notice },
      global: {
        stubs: {
          Icon: true,
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })

    const shop = wrapper.get('[data-test="card-shop-open"]')
    expect(shop.attributes('href')).toBe('https://pay.ldxp.cn/shop/8AJ8M782')
    expect(shop.attributes('target')).toBe('_blank')
    expect(shop.attributes('rel')).toContain('noopener')
    expect(wrapper.get('#inline-redeem-code-card-shop').exists()).toBe(true)
    expect(wrapper.text()).toContain('$12.50')
    expect(wrapper.text()).not.toContain('payment.createOrder')
  })

  it('does not invent a shop button when the notice has no url', () => {
    const wrapper = mount(CardShopRedeemPanel, {
      props: { notice: '请联系客服购买兑换码' },
      global: {
        stubs: {
          Icon: true,
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })

    expect(wrapper.find('[data-test="card-shop-open"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('payment.cardShop.missingShop')
    expect(wrapper.get('#inline-redeem-code-card-shop').exists()).toBe(true)
  })
})
