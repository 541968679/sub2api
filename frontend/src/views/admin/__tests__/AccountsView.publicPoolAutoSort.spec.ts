import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getBatchQualityStats,
  getPublicScheduleQualityBatch,
  getOpenAIOauthFleetUsage,
  reorderAccountsAutoSort,
  getAllProxies,
  getAllGroups,
  showInfo,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getPublicScheduleQualityBatch: vi.fn(),
  getOpenAIOauthFleetUsage: vi.fn(),
  reorderAccountsAutoSort: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  showInfo: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getBatchQualityStats,
      getPublicScheduleQualityBatch,
      getOpenAIOauthFleetUsage,
      reorderAccountsAutoSort,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: {
      getAll: getAllProxies,
      getAllWithCount: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showInfo
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
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

const DataTableStub = {
  props: ['columns', 'data', 'loading'],
  template: `
    <div data-test="data-table" :data-loading="loading ? 'true' : 'false'">
      <div v-for="row in data" :key="row.id" :data-test="'account-row-' + row.id"></div>
    </div>
  `
}

function tableListCalls() {
  return listAccounts.mock.calls.filter((call) => Number(call[1] ?? 20) < 1000)
}

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        HelpTooltip: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: { template: '<div></div>' },
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
        AccountStabilityDialog: true,
        PublicScheduleQualityGlobalCard: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountQualityCell: true,
        AccountGroupsCell: true,
        AccountUserScheduleCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  })
}

function accountRow(
  id: number,
  extra: Record<string, unknown> = {},
  overrides: Record<string, unknown> = {}
) {
  return {
    id,
    name: `acc-${id}`,
    platform: 'openai',
    type: 'apikey',
    status: 'active',
    schedulable: true,
    public_schedulable: true,
    fallback_only: false,
    concurrency: 1,
    current_concurrency: 0,
    priority: 10,
    upstream_rate_multiplier: id === 1 ? 0.1 : 0.5,
    extra,
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    ...overrides
  }
}

const qualityView = {
  overlay: { enabled: false },
  resolved: {
    enabled: false,
    ttft_window_n: 20,
    success_window_n: 20,
    cooldown_minutes: 15,
    soft_cooldown: false
  },
  state: 'selectable',
  will_cool: false
}

function mockList(items: unknown[]) {
  listAccounts.mockResolvedValue({
    items,
    total: items.length,
    page: 1,
    page_size: 20,
    pages: 1
  })
  listWithEtag.mockResolvedValue({
    notModified: false,
    etag: 'etag-1',
    data: {
      items,
      total: items.length,
      page: 1,
      page_size: 20,
      pages: 1
    }
  })
}

describe('admin AccountsView public pool auto-sort', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.useFakeTimers()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getBatchQualityStats.mockReset()
    getPublicScheduleQualityBatch.mockReset()
    getOpenAIOauthFleetUsage.mockReset()
    reorderAccountsAutoSort.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    showInfo.mockReset()
    showSuccess.mockReset()
    showError.mockReset()

    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getBatchQualityStats.mockResolvedValue({ stats: {} })
    getPublicScheduleQualityBatch.mockResolvedValue({
      views: {
        '1': qualityView,
        '2': qualityView,
        '9': qualityView
      }
    })
    getOpenAIOauthFleetUsage.mockResolvedValue(null)
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
    reorderAccountsAutoSort.mockResolvedValue(undefined)
    mockList([accountRow(1), accountRow(2)])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('writes reserved-band order for the current filter and skips pinned ids', async () => {
    mockList([
      accountRow(2, {}, { upstream_rate_multiplier: 0.5 }),
      accountRow(1, {}, { upstream_rate_multiplier: 0.1 }),
      accountRow(9, { list_pinned: true, list_order: 9_000_000_000_000 }, { upstream_rate_multiplier: 0.01 })
    ])
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="accounts-manual-sort"]').trigger('click')
    await flushPromises()

    expect(reorderAccountsAutoSort).toHaveBeenCalledTimes(1)
    const payload = reorderAccountsAutoSort.mock.calls[0][0] as number[]
    expect(payload).toEqual([1, 2])
    expect(payload).not.toContain(9)
    expect(JSON.stringify(reorderAccountsAutoSort.mock.calls[0])).not.toContain('priority')
    expect(showSuccess).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not write when the reserved band is already correct', async () => {
    mockList([
      accountRow(1, { list_order: 2 }, { upstream_rate_multiplier: 0.1 }),
      accountRow(2, { list_order: 1 }, { upstream_rate_multiplier: 0.5 })
    ])
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="accounts-manual-sort"]').trigger('click')
    await flushPromises()

    expect(reorderAccountsAutoSort).not.toHaveBeenCalled()
    expect(showInfo).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('persists the auto-sort toggle and does not write without auto-refresh', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="accounts-interval-auto-sort"]').trigger('click')
    expect(JSON.parse(localStorage.getItem('account-auto-sort') ?? '{}')).toEqual({ enabled: true })

    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()
    expect(reorderAccountsAutoSort).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="accounts-interval-auto-sort"]').trigger('click')
    expect(JSON.parse(localStorage.getItem('account-auto-sort') ?? '{}')).toEqual({ enabled: false })
    wrapper.unmount()
  })

  it('does not auto-sort when incremental refresh fails', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    localStorage.setItem('account-auto-sort', JSON.stringify({ enabled: true }))
    listWithEtag.mockRejectedValue(new Error('etag refresh failed'))
    const wrapper = mountView()
    await flushPromises()

    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()

    expect(listWithEtag).toHaveBeenCalled()
    expect(reorderAccountsAutoSort).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('silently auto-sorts after a successful incremental refresh', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    localStorage.setItem('account-auto-sort', JSON.stringify({ enabled: true }))
    const wrapper = mountView()
    await flushPromises()

    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()

    expect(listWithEtag).toHaveBeenCalled()
    expect(reorderAccountsAutoSort).toHaveBeenCalledTimes(1)
    expect(showSuccess).not.toHaveBeenCalled()
    expect(tableListCalls()).toHaveLength(1)
    expect(wrapper.get('[data-test="data-table"]').attributes('data-loading')).toBe('false')
    expect(wrapper.find('[data-test="account-row-1"]').exists()).toBe(true)

    const countdownBeforeTick = wrapper.get('[data-testid="accounts-interval-auto-sort"]').text()
    expect(countdownBeforeTick).toContain('"seconds":5')
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    const countdownAfterTick = wrapper.get('[data-testid="accounts-interval-auto-sort"]').text()
    expect(countdownAfterTick).toContain('"seconds":')
    expect(countdownAfterTick).not.toMatch(/"seconds":1[45]/)
    expect(reorderAccountsAutoSort).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('keeps the current page after manual sort instead of reload()', async () => {
    const wrapper = mountView()
    await flushPromises()
    listAccounts.mockClear()
    listWithEtag.mockClear()

    await wrapper.get('[data-testid="accounts-manual-sort"]').trigger('click')
    await flushPromises()

    expect(reorderAccountsAutoSort).toHaveBeenCalledTimes(1)
    expect(tableListCalls()).toHaveLength(0)
    expect(listWithEtag).toHaveBeenCalled()
    expect(wrapper.get('[data-test="data-table"]').attributes('data-loading')).toBe('false')
    expect(wrapper.find('[data-test="account-row-1"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('asks the operator to narrow filters when more than 2000 accounts match', async () => {
    const overflow = Array.from({ length: 2001 }, (_, index) =>
      accountRow(index + 1, {}, { upstream_rate_multiplier: 0.15 })
    )
    listAccounts.mockImplementation((_page: number, pageSize: number) => {
      if (pageSize >= 1000) {
        return Promise.resolve({
          items: overflow,
          total: overflow.length,
          page: 1,
          page_size: pageSize,
          pages: 1
        })
      }
      return Promise.resolve({
        items: [overflow[0]],
        total: overflow.length,
        page: 1,
        page_size: pageSize,
        pages: 3
      })
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="accounts-manual-sort"]').trigger('click')
    await flushPromises()

    expect(reorderAccountsAutoSort).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalled()
    wrapper.unmount()
  })
})
