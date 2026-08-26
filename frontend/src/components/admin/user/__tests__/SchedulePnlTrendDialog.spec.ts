import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SchedulePnlTrendDialog from '../SchedulePnlTrendDialog.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getSmartSchedulePnlTrend: vi.fn().mockResolvedValue({ range: '24h', granularity: 'hour', points: [] })
    }
  }
}))

vi.mock('vue-chartjs', () => ({
  Line: { name: 'LineChart', template: '<div />' }
}))

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  CategoryScale: {},
  LinearScale: {},
  PointElement: {},
  LineElement: {},
  Tooltip: {},
  Legend: {},
  Filler: {}
}))

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show', 'title', 'width'],
    template: '<div v-if="show"><slot /></div>'
  }
}))

vi.mock('@/components/common/EmptyState.vue', () => ({
  default: { template: '<div data-testid="schedule-pnl-empty" />' }
}))

vi.mock('@/components/common/LoadingSpinner.vue', () => ({
  default: { template: '<div />' }
}))

function apikeyAccount(extra: Record<string, unknown>): Account {
  return {
    id: 12,
    name: 'tokenbits-012',
    platform: 'openai',
    type: 'apikey',
    extra
  } as Account
}

describe('SchedulePnlTrendDialog', () => {
  it('hides wallet/subscription split on user-level dialog', () => {
    const wrapper = mount(SchedulePnlTrendDialog, {
      props: { show: true, userId: 9, title: 'user' }
    })
    expect(wrapper.find('[data-testid="schedule-pnl-balance-split"]').exists()).toBe(false)
  })

  it('shows wallet and subscription when the pair account has both extras', () => {
    const wrapper = mount(SchedulePnlTrendDialog, {
      props: {
        show: true,
        userId: 9,
        accountId: 12,
        account: apikeyAccount({
          upstream_balance_usd: 336.61,
          upstream_balance_wallet_usd: 0,
          upstream_balance_subscription_usd: 336.61
        })
      }
    })
    expect(wrapper.get('[data-testid="schedule-pnl-balance-wallet"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-testid="schedule-pnl-balance-subscription"]').text()).toContain('$336.61')
  })

  it('hides wallet/subscription split for oauth pair accounts', () => {
    const wrapper = mount(SchedulePnlTrendDialog, {
      props: {
        show: true,
        userId: 9,
        accountId: 12,
        account: {
          id: 12,
          name: 'oauth-1',
          platform: 'openai',
          type: 'oauth',
          extra: {
            upstream_balance_usd: 10,
            upstream_balance_wallet_usd: 10
          }
        } as Account
      }
    })
    expect(wrapper.find('[data-testid="schedule-pnl-balance-split"]').exists()).toBe(false)
  })

  it('hides the subscription row when extra has no subscription remaining', () => {
    const wrapper = mount(SchedulePnlTrendDialog, {
      props: {
        show: true,
        userId: 9,
        accountId: 12,
        account: apikeyAccount({
          upstream_balance_usd: 62.57,
          upstream_balance_wallet_usd: 62.57
        })
      }
    })
    expect(wrapper.get('[data-testid="schedule-pnl-balance-wallet"]').text()).toContain('$62.57')
    expect(wrapper.find('[data-testid="schedule-pnl-balance-subscription"]').exists()).toBe(false)
  })
})
