import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

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
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <button data-test="sort-last-used" @click="$emit('sort', 'last_used_at', 'desc')">sort</button>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-last_used_at" :value="row.last_used_at" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
}

describe('admin UsersView', () => {
  const mountedWrappers: VueWrapper[] = []

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

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

const mountUsersView = () => {
  const wrapper = mount(UsersView, {
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
        AccountQualityCell: true,
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
  mountedWrappers.push(wrapper)
  return wrapper
}

  it('shows active, used, and created activity columns in order and requests last_used_at sort', async () => {
    const wrapper = mountUsersView()

    await flushPromises()

    const columns = wrapper.get('[data-test="columns"]').text()
    const visibleColumns = columns.split(',')
    expect(visibleColumns.slice(-4, -1)).toEqual(['last_active_at', 'last_used_at', 'created_at'])
    expect(visibleColumns).not.toContain('last_login_at')
    const concurrencyIdx = visibleColumns.indexOf('concurrency')
    expect(concurrencyIdx).toBeGreaterThanOrEqual(0)
    expect(visibleColumns[concurrencyIdx + 1]).toBe('quality_ttft')
    expect(visibleColumns[concurrencyIdx + 2]).toBe('quality_success_rate')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('exposes auto-refresh control and persists enable preference', async () => {
    const wrapper = mountUsersView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.users.autoRefresh')

    const autoRefreshButton = wrapper
      .findAll('button')
      .find((btn) => btn.attributes('title') === 'admin.users.autoRefresh')
    expect(autoRefreshButton).toBeTruthy()
    await autoRefreshButton!.trigger('click')
    await flushPromises()

    const enableButton = wrapper
      .findAll('button')
      .find((btn) => btn.text().includes('admin.users.enableAutoRefresh'))
    expect(enableButton).toBeTruthy()
    await enableButton!.trigger('click')
    await flushPromises()

    const saved = JSON.parse(localStorage.getItem('user-auto-refresh') || '{}') as {
      enabled?: boolean
      interval_seconds?: number
    }
    expect(saved.enabled).toBe(true)
    expect(saved.interval_seconds).toBe(5)
  })

  it('keeps burn-rate off by default and only fetches when enabled', async () => {
    const wrapper = mountUsersView()
    await flushPromises()

    expect(getBatchUsersBurnRate).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="columns"]').text().split(',')).not.toContain('burn_rate')

    const burnRateButton = wrapper
      .findAll('button')
      .find((btn) => btn.attributes('title') === 'admin.users.burnRateToggleTip')
    expect(burnRateButton).toBeTruthy()
    await burnRateButton!.trigger('click')
    await flushPromises()

    expect(localStorage.getItem('user-burn-rate-enabled')).toBe('1')
    expect(getBatchUsersBurnRate).toHaveBeenCalled()
    expect(wrapper.get('[data-test="columns"]').text().split(',')).toContain('burn_rate')
  })

  it('switches burn-rate unit between hour and minute without re-fetch', async () => {
    const wrapper = mountUsersView()
    await flushPromises()

    const burnRateButton = wrapper
      .findAll('button')
      .find((btn) => btn.attributes('title') === 'admin.users.burnRateToggleTip')
    await burnRateButton!.trigger('click')
    await flushPromises()

    const callsAfterEnable = getBatchUsersBurnRate.mock.calls.length

    const minuteButton = wrapper
      .findAll('button')
      .find((btn) => btn.attributes('title') === 'admin.users.burnRateUnitMinuteTip')
    expect(minuteButton).toBeTruthy()
    await minuteButton!.trigger('click')
    await flushPromises()

    expect(localStorage.getItem('user-burn-rate-unit')).toBe('minute')
    expect(getBatchUsersBurnRate.mock.calls.length).toBe(callsAfterEnable)

    const hourButton = wrapper
      .findAll('button')
      .find((btn) => btn.attributes('title') === 'admin.users.burnRateUnitHourTip')
    await hourButton!.trigger('click')
    await flushPromises()

    expect(localStorage.getItem('user-burn-rate-unit')).toBe('hour')
  })

  const waitForDeferredSecondaryLoad = async () => {
    // loadUsers defers secondary fetches by 50ms so the table can paint first.
    await new Promise((resolve) => setTimeout(resolve, 60))
    await flushPromises()
  }

  it('fetches quality stats by default and skips when both quality columns are hidden', async () => {
    const wrapper = mountUsersView()
    await flushPromises()
    await vi.waitFor(() => {
      expect(getBatchQualityStats).toHaveBeenCalledWith([42])
    })

    const columnButton = wrapper
      .findAll('button')
      .find((btn) => btn.attributes('title') === 'admin.users.columnSettings')
    expect(columnButton).toBeTruthy()
    await columnButton!.trigger('click')
    await flushPromises()

    const ttftToggle = wrapper
      .findAll('button')
      .find((btn) => btn.text().includes('admin.users.columns.qualityTtft'))
    const successToggle = wrapper
      .findAll('button')
      .find((btn) => btn.text().includes('admin.users.columns.qualitySuccessRate'))
    expect(ttftToggle).toBeTruthy()
    expect(successToggle).toBeTruthy()
    await ttftToggle!.trigger('click')
    await successToggle!.trigger('click')
    await flushPromises()

    const callsAfterHide = getBatchQualityStats.mock.calls.length
    expect(wrapper.get('[data-test="columns"]').text().split(',')).not.toContain('quality_ttft')
    expect(wrapper.get('[data-test="columns"]').text().split(',')).not.toContain('quality_success_rate')

    await ttftToggle!.trigger('click')
    await flushPromises()
    await vi.waitFor(() => {
      expect(getBatchQualityStats.mock.calls.length).toBeGreaterThan(callsAfterHide)
    })
    expect(wrapper.get('[data-test="columns"]').text().split(',')).toContain('quality_ttft')
  })

  it('does not fetch quality stats when both columns start hidden', async () => {
    localStorage.setItem(
      'user-hidden-columns',
      JSON.stringify(['notes', 'groups', 'subscriptions', 'usage', 'quality_ttft', 'quality_success_rate'])
    )
    mountUsersView()
    await flushPromises()
    await waitForDeferredSecondaryLoad()

    expect(getBatchQualityStats).not.toHaveBeenCalled()
  })

  it('opens the inspect dialog instead of a new tab for usage and errors', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mountUsersView()
    await flushPromises()

    expect(wrapper.get('[data-testid="user-view-error-requests"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="user-view-usage"]').exists()).toBe(true)

    await wrapper.get('[data-testid="user-view-usage"]').trigger('click')
    expect((wrapper.vm as any).inspectOpen).toBe(true)
    expect((wrapper.vm as any).inspectTab).toBe('usage')
    expect((wrapper.vm as any).inspectSubjectId).toBe(42)
    expect(openSpy).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="user-view-error-requests"]').trigger('click')
    expect((wrapper.vm as any).inspectOpen).toBe(true)
    expect((wrapper.vm as any).inspectTab).toBe('errors')
    expect(openSpy).not.toHaveBeenCalled()
    openSpy.mockRestore()
  })
})
