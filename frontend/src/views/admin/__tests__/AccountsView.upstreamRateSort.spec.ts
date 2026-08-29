import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const { listAccounts, listWithEtag, getBatchTodayStats, getBatchQualityStats, getPublicScheduleQualityBatch, getAllProxies, getAllGroups } =
  vi.hoisted(() => ({
    listAccounts: vi.fn(),
    listWithEtag: vi.fn(),
    getBatchTodayStats: vi.fn(),
    getBatchQualityStats: vi.fn(),
    getPublicScheduleQualityBatch: vi.fn(),
    getAllProxies: vi.fn(),
    getAllGroups: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getBatchQualityStats,
      getPublicScheduleQualityBatch,
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

const DataTableStub = {
  props: ['columns'],
  emits: ['sort'],
  template: `
    <div data-test="data-table">
      <div data-test="upstream-sortable">
        {{ columns.find((col) => col.key === 'upstream_rate_multiplier')?.sortable ? '1' : '0' }}
      </div>
      <button data-test="sort-upstream-asc" type="button" @click="$emit('sort', 'upstream_rate_multiplier', 'asc')">asc</button>
      <button data-test="sort-upstream-desc" type="button" @click="$emit('sort', 'upstream_rate_multiplier', 'desc')">desc</button>
    </div>
  `
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
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountQualityCell: true,
        AccountStabilityDialog: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  })
}

describe('admin AccountsView upstream rate column sort', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getBatchQualityStats.mockReset()
    getPublicScheduleQualityBatch.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getBatchQualityStats.mockResolvedValue({ stats: {} })
    getPublicScheduleQualityBatch.mockResolvedValue({ items: [] })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('marks upstream rate sortable and reloads with that sort_by', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="upstream-sortable"]').text()).toBe('1')
    expect(listAccounts).toHaveBeenCalled()

    await wrapper.get('[data-test="sort-upstream-asc"]').trigger('click')
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledWith(
      1,
      expect.any(Number),
      expect.objectContaining({ sort_by: 'upstream_rate_multiplier', sort_order: 'asc' }),
      expect.anything()
    )

    await wrapper.get('[data-test="sort-upstream-desc"]').trigger('click')
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledWith(
      1,
      expect.any(Number),
      expect.objectContaining({ sort_by: 'upstream_rate_multiplier', sort_order: 'desc' }),
      expect.anything()
    )
    wrapper.unmount()
  })
})
