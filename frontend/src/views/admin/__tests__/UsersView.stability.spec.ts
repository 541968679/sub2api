import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UsersView from '../UsersView.vue'

const {
  listUsers,
  getAllGroups,
  getBatchUsersUsage,
  getBatchUsersBurnRate,
  getBatchQualityStats,
  getQualityHistory,
  getQualityHardCloseSettings,
  listEnabledDefinitions,
  getBatchUserAttributes
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  getBatchUsersBurnRate: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getQualityHistory: vi.fn(),
  getQualityHardCloseSettings: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn()
}))

const accountsGetQualityHistory = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      toggleStatus: vi.fn(),
      delete: vi.fn(),
      getBatchQualityStats,
      getQualityHistory
    },
    groups: {
      getAll: getAllGroups
    },
    dashboard: {
      getBatchUsersUsage,
      getBatchUsersBurnRate
    },
    userAttributes: {
      listEnabledDefinitions,
      getBatchUserAttributes
    },
    accounts: {
      getQualityHistory: accountsGetQualityHistory,
      getQualityHardClose: vi.fn(),
      getBatchQualityStats: vi.fn()
    },
    settings: {
      getQualityHardCloseSettings
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
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

const createAdminUser = (): AdminUser => ({
  id: 42,
  username: 'scoped-user',
  email: 'scoped@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-17T00:00:00Z',
  updated_at: '2026-04-17T00:00:00Z',
  notes: '',
  last_active_at: '2026-04-16T02:00:00Z',
  last_used_at: '2026-04-17T02:00:00Z',
  current_concurrency: 0
})

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div v-for="row in data" :key="row.id" :data-test="'user-row-' + row.id">
        <div data-test="quality-cell">
          <slot name="cell-quality_ttft" :row="row" />
        </div>
      </div>
    </div>
  `
}

function mountView() {
  return mount(UsersView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        Pagination: true,
        ConfirmDialog: true,
        EmptyState: true,
        GroupBadge: true,
        Select: true,
        UserAttributesConfigModal: true,
        UserConcurrencyCell: true,
        HelpTooltip: true,
        UserCreateModal: true,
        UserEditModal: true,
        UserApiKeysModal: true,
        UserAllowedGroupsModal: true,
        UserModelPricingModal: true,
        UserPlatformQuotaModal: true,
        UserBalanceModal: true,
        UserBalanceHistoryModal: true,
        UserBalanceHistoryManageModal: true,
        GroupReplaceModal: true,
        UsageErrorInspectDialog: true,
        UserSchedulePnlCell: true,
        SchedulePnlTrendDialog: true,
        UserQualityDialog: {
          props: ['show', 'userId', 'title'],
          template:
            '<div v-if="show" data-test="user-quality-dialog" :data-user-id="userId">{{ title }}</div>'
        },
        Icon: true,
        Teleport: true
      }
    }
  })
}

describe('admin UsersView user quality cell opens a user-scoped dialog', () => {
  beforeEach(() => {
    localStorage.clear()
    listUsers.mockReset()
    getAllGroups.mockReset()
    getBatchUsersUsage.mockReset()
    getBatchUsersBurnRate.mockReset()
    getBatchQualityStats.mockReset()
    getQualityHistory.mockReset()
    getQualityHardCloseSettings.mockReset()
    listEnabledDefinitions.mockReset()
    getBatchUserAttributes.mockReset()
    accountsGetQualityHistory.mockReset()

    listUsers.mockResolvedValue({
      items: [createAdminUser()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getAllGroups.mockResolvedValue([])
    getBatchUsersUsage.mockResolvedValue({ stats: {} })
    getBatchUsersBurnRate.mockResolvedValue({ stats: {} })
    getBatchQualityStats.mockResolvedValue({
      stats: {
        '42': {
          window_n: 20,
          account_quality_window_n: 20,
          success_count: 10,
          error_count: 1,
          success_rate: 0.91,
          failover_error_count: 2,
          failover_error_rate: 0.1,
          avg_ttft_ms: 400,
          p50_ttft_ms: 300,
          p95_ttft_ms: 900,
          max_ttft_ms: 1200,
          ttft_samples: 10
        }
      }
    })
    getQualityHistory.mockResolvedValue({ items: [], from: '2026-08-20T00:00:00Z', to: '2026-08-21T00:00:00Z' })
    getQualityHardCloseSettings.mockResolvedValue({ account_quality_window_n: 20 })
    listEnabledDefinitions.mockResolvedValue([])
    getBatchUserAttributes.mockResolvedValue({ values: {} })
  })

  it('renders one combined clickable quality cell and opens the user dialog', async () => {
    const wrapper = mountView()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 60))
    await flushPromises()

    expect(wrapper.findAll('[data-test="user-quality-cell-button"]')).toHaveLength(1)
    expect(wrapper.find('[data-test="account-quality-cell"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="user-quality-dialog"]').exists()).toBe(false)

    await wrapper.get('[data-test="user-quality-cell-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="user-quality-dialog"]').attributes('data-user-id')).toBe('42')
    expect(accountsGetQualityHistory).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="stability-dialog"]').exists()).toBe(false)
  })
})
