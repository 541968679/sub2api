import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups,
  deleteAccount
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  deleteAccount: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      delete: deleteAccount,
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
  props: ['columns', 'data'],
  template: '<div data-test="data-table"></div>'
}

const AccountBulkActionsBarStub = {
  props: ['selectedIds', 'total', 'selectingAllFiltered'],
  emits: ['delete', 'edit-filtered', 'select-filtered'],
  template: `
    <div>
      <span data-test="selected-count">{{ selectedIds.length }}</span>
      <button data-test="delete-selected" @click="$emit('delete')">delete selected</button>
      <button data-test="edit-filtered" @click="$emit('edit-filtered')">edit filtered</button>
      <button data-test="select-filtered" @click="$emit('select-filtered')">select filtered</button>
    </div>
  `
}

const BulkEditAccountModalStub = {
  props: ['show', 'target'],
  template: '<div data-test="bulk-edit-modal" :data-show="String(show)" :data-target-mode="target?.mode ?? \'\'"></div>'
}

const mountAccountsView = () => mount(AccountsView, {
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
      AccountBulkActionsBar: AccountBulkActionsBarStub,
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
      BulkEditAccountModal: BulkEditAccountModalStub,
      PlatformTypeBadge: true,
      AccountCapacityCell: true,
      AccountStatusIndicator: true,
      AccountTodayStatsCell: true,
      AccountGroupsCell: true,
      AccountUsageCell: true,
      Icon: true
    }
  }
})

describe('admin AccountsView bulk edit scope', () => {
  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    deleteAccount.mockReset()

    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
    deleteAccount.mockResolvedValue({ message: 'ok' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('opens bulk edit in filtered-results mode from the bulk actions dropdown', async () => {
    const wrapper = mountAccountsView()

    await flushPromises()
    await wrapper.get('[data-test="edit-filtered"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-show')).toBe('true')
    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-target-mode')).toBe('filtered')
  })

  it('selects account ids from every filtered page', async () => {
    listAccounts
      .mockResolvedValueOnce({
        items: [],
        total: 3,
        page: 1,
        page_size: 20,
        pages: 1
      })
      .mockResolvedValueOnce({
        items: [
          { id: 1, platform: 'openai', type: 'oauth' },
          { id: 2, platform: 'anthropic', type: 'setup-token' }
        ],
        total: 3,
        page: 1,
        page_size: 1000,
        pages: 2
      })
      .mockResolvedValueOnce({
        items: [
          { id: 3, platform: 'gemini', type: 'oauth' }
        ],
        total: 3,
        page: 2,
        page_size: 1000,
        pages: 2
      })

    const wrapper = mountAccountsView()

    await flushPromises()
    await wrapper.get('[data-test="select-filtered"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenNthCalledWith(2, 1, 1000, expect.objectContaining({ sort_by: 'created_at', sort_order: 'desc' }))
    expect(listAccounts).toHaveBeenNthCalledWith(3, 2, 1000, expect.objectContaining({ sort_by: 'created_at', sort_order: 'desc' }))
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('3')
  })

  it('bulk deletes selected accounts in bounded batches and keeps failures selected', async () => {
    const selectedAccounts = Array.from({ length: 12 }, (_, index) => ({
      id: index + 1,
      platform: 'openai',
      type: 'oauth'
    }))

    listAccounts
      .mockResolvedValueOnce({
        items: [],
        total: selectedAccounts.length,
        page: 1,
        page_size: 20,
        pages: 1
      })
      .mockResolvedValueOnce({
        items: selectedAccounts,
        total: selectedAccounts.length,
        page: 1,
        page_size: 1000,
        pages: 1
      })
      .mockResolvedValue({
        items: [],
        total: 1,
        page: 1,
        page_size: 20,
        pages: 1
      })

    vi.spyOn(window, 'confirm').mockReturnValue(true)

    let activeDeletes = 0
    let maxActiveDeletes = 0
    const completeDeletes: Array<() => void> = []

    deleteAccount.mockImplementation((id: number) => {
      activeDeletes += 1
      maxActiveDeletes = Math.max(maxActiveDeletes, activeDeletes)
      return new Promise((resolve, reject) => {
        completeDeletes.push(() => {
          activeDeletes -= 1
          if (id === 2) {
            reject(new Error('delete failed'))
          } else {
            resolve({ message: 'ok' })
          }
        })
      })
    })

    const wrapper = mountAccountsView()
    await flushPromises()
    await wrapper.get('[data-test="select-filtered"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('12')

    await wrapper.get('[data-test="delete-selected"]').trigger('click')
    await flushPromises()
    const firstBatchCalls = deleteAccount.mock.calls.length
    const firstBatchMaxActive = maxActiveDeletes

    completeDeletes.splice(0).forEach(complete => complete())
    await flushPromises()
    const totalCallsAfterSecondBatchStarts = deleteAccount.mock.calls.length

    completeDeletes.splice(0).forEach(complete => complete())
    await flushPromises()

    expect(firstBatchCalls).toBe(10)
    expect(firstBatchMaxActive).toBeLessThanOrEqual(10)
    expect(totalCallsAfterSecondBatchStarts).toBe(12)
    expect(deleteAccount.mock.calls.map(call => call[0])).toEqual(selectedAccounts.map(account => account.id))
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('1')
  })
})
