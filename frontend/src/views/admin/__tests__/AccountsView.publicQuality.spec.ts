import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getBatchQualityStats,
  getPublicScheduleQualityBatch,
  setPublicScheduleQualityState,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getPublicScheduleQualityBatch: vi.fn(),
  setPublicScheduleQualityState: vi.fn(),
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
      setPublicScheduleQualityState,
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
      <div data-test="column-keys">{{ columns.map((col) => col.key).join(',') }}</div>
      <div v-for="row in data" :key="row.id" :data-test="'account-row-' + row.id">
        <slot name="cell-public_quality" :row="row" />
      </div>
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

const baseAccount = {
  id: 7,
  name: 'public-q',
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

describe('admin AccountsView public quality column', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getBatchQualityStats.mockReset()
    getPublicScheduleQualityBatch.mockReset()
    setPublicScheduleQualityState.mockReset()
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
    getPublicScheduleQualityBatch.mockResolvedValue({
      views: {
        '7': {
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
      }
    })
    setPublicScheduleQualityState.mockResolvedValue({
      overlay: { enabled: false },
      resolved: {
        enabled: false,
        ttft_window_n: 20,
        success_window_n: 20,
        cooldown_minutes: 15,
        soft_cooldown: false
      },
      state: 'paused',
      will_cool: false
    })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('loads the public-quality column after quality and lets the switch persist', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="column-keys"]').text().split(',')).toContain('public_quality')
    expect(getPublicScheduleQualityBatch).toHaveBeenCalledWith([7])
    expect(wrapper.get('[data-testid="account-public-quality"]').attributes('data-state')).toBe('selectable')

    await wrapper.get('[data-testid="smart-schedule-admission-switch"]').trigger('click')
    await flushPromises()
    const paused = document.querySelector('[data-testid="smart-schedule-admission-paused"]') as HTMLButtonElement
    expect(paused).toBeTruthy()
    paused.click()
    await flushPromises()
    expect(setPublicScheduleQualityState).toHaveBeenCalledWith(7, 'paused')
    expect(wrapper.get('[data-testid="account-public-quality"]').attributes('data-state')).toBe('paused')
    wrapper.unmount()
  })
})
