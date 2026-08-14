import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getBatchQualityStats,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getBatchQualityStats: vi.fn(),
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
      getQualityHistory: vi.fn(),
      getQualityHardClose: vi.fn(),
      updateQualityHardClose: vi.fn(),
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
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div v-for="row in data" :key="row.id" :data-test="'account-row-' + row.id">
        <div data-test="quality-ttft">
          <slot name="cell-quality_ttft" :row="row" />
        </div>
        <div data-test="quality-success">
          <slot name="cell-quality_success_rate" :row="row" />
        </div>
      </div>
    </div>
  `
}

const StabilityDialogStub = {
  props: ['show', 'account'],
  template: '<div v-if="show" data-test="stability-dialog">{{ account?.id }}:{{ account?.name }}</div>'
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
        AccountStabilityDialog: StabilityDialogStub,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUserScheduleCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  })
}

const baseAccount = {
  id: 7,
  name: 'stable-acc',
  platform: 'openai',
  type: 'apikey',
  status: 'active',
  schedulable: true,
  concurrency: 1,
  priority: 0,
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  temp_unschedulable_until: null,
  temp_unschedulable_reason: null
}

describe('admin AccountsView stability window', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getBatchQualityStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listAccounts.mockResolvedValue({
      items: [baseAccount],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getBatchQualityStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('opens one shared stability dialog from either quality cell', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="stability-dialog"]').exists()).toBe(false)

    const buttons = wrapper.findAll('[data-test="account-quality-cell-button"]')
    expect(buttons).toHaveLength(2)

    await buttons[0].trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="stability-dialog"]').text()).toBe('7:stable-acc')

    await buttons[1].trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="stability-dialog"]').text()).toBe('7:stable-acc')
  })
})
