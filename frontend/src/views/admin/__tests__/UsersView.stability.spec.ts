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
  listEnabledDefinitions,
  getBatchUserAttributes
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  getBatchUsersBurnRate: vi.fn(),
  getBatchQualityStats: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      toggleStatus: vi.fn(),
      delete: vi.fn(),
      getBatchQualityStats
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
      getQualityHistory: vi.fn(),
      getQualityHardClose: vi.fn()
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
        Icon: true,
        Teleport: true
      }
    }
  })
}

describe('admin UsersView quality cells stay read-only', () => {
  beforeEach(() => {
    localStorage.clear()
    listUsers.mockReset()
    getAllGroups.mockReset()
    getBatchUsersUsage.mockReset()
    getBatchUsersBurnRate.mockReset()
    getBatchQualityStats.mockReset()
    listEnabledDefinitions.mockReset()
    getBatchUserAttributes.mockReset()

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
    getBatchQualityStats.mockResolvedValue({ stats: {} })
    listEnabledDefinitions.mockResolvedValue([])
    getBatchUserAttributes.mockResolvedValue({ values: {} })
  })

  it('does not make first-token or success-rate cells open a stability window', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('[data-test="account-quality-cell"]')).toHaveLength(2)
    expect(wrapper.find('[data-test="account-quality-cell-button"]').exists()).toBe(false)
    expect(wrapper.find('button[aria-label="admin.accounts.stability.openAria"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="stability-dialog"]').exists()).toBe(false)

    await wrapper.get('[data-test="account-quality-cell"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="stability-dialog"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="account-quality-cell-button"]').exists()).toBe(false)
  })
})
