import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const { listAccounts } = vi.hoisted(() => ({
  listAccounts: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag: vi.fn().mockResolvedValue({ notModified: true, etag: null, data: null }),
      getBatchTodayStats: vi.fn().mockResolvedValue({ stats: {} }),
      getBatchQualityStats: vi.fn().mockResolvedValue({ stats: {} }),
      getPublicScheduleQualityBatch: vi.fn().mockResolvedValue({ views: {} })
    },
    proxies: {
      getAll: vi.fn().mockResolvedValue([]),
      getAllWithCount: vi.fn().mockResolvedValue([])
    },
    groups: {
      getAll: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const nowMs = Date.now()
const sample = (hoursAgo: number, value: number, kind = 'balance_usd') => ({
  t: new Date(nowMs - hoursAgo * 3_600_000).toISOString(),
  v: value,
  kind
})
const spanHours = 0.125
const usdSamples = (startValue: number, dropPerHour: number) => [
  sample(spanHours, startValue),
  sample(0, startValue - dropPerHour * spanHours)
]

const mountAccountsView = () =>
  mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: { template: '<div data-test="data-table"></div>' },
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: true,
        AccountBulkActionsBar: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        UnbindSubscriptionGroupsDialog: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountQualityCell: true,
        AccountStabilityDialog: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        AccountInlineNumberCell: true,
        AccountUserScheduleCell: true,
        PublicScheduleQualityStateCell: true,
        PublicScheduleQualityGlobalCard: true,
        UpdateRefreshTokenModal: true,
        UsageErrorInspectDialog: true,
        HelpTooltip: true,
        Icon: true
      }
    }
  })

describe('admin AccountsView burn rate total', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset()
  })

  it('shows the summed balance burn rate above the account list', async () => {
    listAccounts.mockResolvedValue({
      items: [
        { id: 1, name: 'usd-a', platform: 'openai', type: 'apikey', extra: { burn_samples: usdSamples(100, 10) } },
        { id: 2, name: 'usd-b', platform: 'openai', type: 'apikey', extra: { burn_samples: usdSamples(40, 2.5) } },
        {
          id: 3,
          name: 'cny',
          platform: 'openai',
          type: 'apikey',
          extra: {
            upstream_balance_currency: 'CNY',
            burn_samples: [sample(spanHours, 30, 'balance_cny'), sample(0, 30 - 3 * spanHours, 'balance_cny')]
          }
        }
      ],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const banner = wrapper.get('[data-testid="account-burn-rate-total"]')
    expect(banner.text()).toContain('admin.accounts.burnRateTotal')
    expect(banner.text()).not.toContain('admin.accounts.burnRateTotalPage')
    expect(wrapper.get('[data-testid="account-burn-rate-total-rate"]').attributes('data-rate')).toBe('15.5')
    expect(wrapper.find('[data-testid="account-burn-rate-total-cny"]').exists()).toBe(false)
  })

  it('labels the total as the current page when the filter has more pages', async () => {
    listAccounts.mockResolvedValue({
      items: [
        { id: 1, name: 'usd-a', platform: 'openai', type: 'apikey', extra: { burn_samples: usdSamples(20, 1) } }
      ],
      total: 40,
      page: 1,
      page_size: 20,
      pages: 2
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    expect(wrapper.get('[data-testid="account-burn-rate-total"]').text()).toContain(
      'admin.accounts.burnRateTotalPage'
    )
    expect(wrapper.get('[data-testid="account-burn-rate-total-rate"]').attributes('data-rate')).toBe('1')
  })

  it('keeps the total bar on screen when the page has no usable balance rate', async () => {
    listAccounts.mockResolvedValue({
      items: [{ id: 1, name: 'plain', platform: 'openai', type: 'apikey', extra: {} }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const banner = wrapper.get('[data-testid="account-burn-rate-total"]')
    expect(banner.text()).toContain('admin.accounts.burnRateTotal')
    expect(wrapper.get('[data-testid="account-burn-rate-total-rate"]').attributes('data-rate')).toBe('0')
  })
})
