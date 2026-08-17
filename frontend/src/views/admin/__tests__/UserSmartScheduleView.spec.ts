import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const apiMocks = vi.hoisted(() => ({
  getById: vi.fn(),
  listUsers: vi.fn(),
  getUserBatchQualityStats: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  getBatchUsersBurnRate: vi.fn(),
  getSmartSchedule: vi.fn(),
  updateSmartSchedule: vi.fn(),
  updateSmartScheduleSortOrder: vi.fn(),
  copySmartSchedule: vi.fn(),
  listAccounts: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getQualityHardCloseSettings: vi.fn(),
  updateQualityHardCloseSettings: vi.fn(),
  updateAccount: vi.fn(),
  moveAccountToTop: vi.fn(),
  resumeSmartSchedule: vi.fn()
}))

const tableMocks = vi.hoisted(() => ({
  setSort: vi.fn()
}))

const autoSortMocks = vi.hoisted(() => ({
  sortSmartSchedulePoolMembers: vi.fn()
}))

vi.mock('@/composables/smartSchedulePoolAutoSort', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/composables/smartSchedulePoolAutoSort')>()
  autoSortMocks.sortSmartSchedulePoolMembers.mockImplementation(actual.sortSmartSchedulePoolMembers)
  return {
    ...actual,
    sortSmartSchedulePoolMembers: (
      ...args: Parameters<typeof actual.sortSmartSchedulePoolMembers>
    ) => autoSortMocks.sortSmartSchedulePoolMembers(...args)
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getById: apiMocks.getById,
      list: apiMocks.listUsers,
      getBatchQualityStats: apiMocks.getUserBatchQualityStats,
      getSmartSchedule: apiMocks.getSmartSchedule,
      updateSmartSchedule: apiMocks.updateSmartSchedule,
      updateSmartScheduleSortOrder: apiMocks.updateSmartScheduleSortOrder,
      copySmartSchedule: apiMocks.copySmartSchedule
    },
    dashboard: {
      getBatchUsersUsage: apiMocks.getBatchUsersUsage,
      getBatchUsersBurnRate: apiMocks.getBatchUsersBurnRate
    },
    accounts: {
      list: apiMocks.listAccounts,
      getBatchQualityStats: apiMocks.getBatchQualityStats,
      getBatchTodayStats: apiMocks.getBatchTodayStats,
      update: apiMocks.updateAccount,
      setSchedulable: vi.fn(),
      resumeSmartSchedule: apiMocks.resumeSmartSchedule,
      moveAccountToTop: apiMocks.moveAccountToTop
    },
    groups: { getAll: vi.fn().mockResolvedValue([]) },
    proxies: { getAllWithCount: vi.fn().mockResolvedValue([]) },
    settings: {
      getQualityHardCloseSettings: apiMocks.getQualityHardCloseSettings,
      updateQualityHardCloseSettings: apiMocks.updateQualityHardCloseSettings
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '99' } }),
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (params) return key.replace(/\{(\w+)\}/g, (_, k) => params[k] ?? '')
        return key
      }
    })
  }
})

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<div><slot /></div>' }
}))
vi.mock('@/components/icons/Icon.vue', () => ({ default: { template: '<span />' } }))
vi.mock('@/components/common/DataTable.vue', () => ({
  default: {
    props: ['columns', 'data', 'loading'],
    computed: {
      isUserRow(): boolean {
        return (this.columns ?? []).some((col: { key: string }) => col.key === 'quality_success_rate')
      }
    },
    methods: {
      setSort(key: string, order: 'asc' | 'desc') {
        tableMocks.setSort(key, order)
      }
    },
    template: `
      <div :data-testid="isUserRow ? 'smart-schedule-user-table' : 'smart-schedule-pool-table'">
        <div :data-testid="isUserRow ? 'smart-schedule-user-headers' : 'smart-schedule-pool-headers'">
          <span
            v-for="col in columns"
            :key="col.key"
            :data-column="col.key"
            :data-sortable="col.sortable ? 'true' : 'false'"
          >{{ col.label }}</span>
        </div>
        <div v-for="row in data" :key="row.id">
          <slot name="cell-select" :row="row" />
          <slot name="cell-name" :row="row" :value="row.name" />
          <slot name="cell-sort_order" :row="row" :value="row.sort_order" />
          <slot name="cell-priority" :row="row" :value="row.priority" />
          <slot name="cell-email" :row="row" :value="row.email" />
          <slot name="cell-username" :row="row" :value="row.username" />
          <slot name="cell-balance" :row="row" :value="row.balance" />
          <slot name="cell-burn_rate" :row="row" />
          <slot name="cell-concurrency" :row="row" />
          <slot name="cell-usage" :row="row" />
          <slot name="cell-groups" :row="row" />
          <slot name="cell-smart_schedule" :row="row" />
          <slot name="cell-pair_cap" :row="row" />
          <slot name="cell-admission" :row="row" />
          <slot name="cell-quality_ttft" :row="row" />
          <slot name="cell-quality_success_rate" :row="row" />
          <slot name="cell-status" :row="row" :value="row.status" />
          <slot name="cell-actions" :row="row" />
        </div>
      </div>
    `
  }
}))
vi.mock('@/components/common/HelpTooltip.vue', () => ({ default: { template: '<span />' } }))
vi.mock('@/components/common/ConfirmDialog.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/PlatformTypeBadge.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountStatusIndicator.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountQualityCell.vue', () => ({
  default: {
    props: ['clickable', 'mode', 'stats', 'loading', 'error'],
    template: `
      <button
        v-if="mode === 'combined'"
        data-testid="account-quality-cell-button"
        @click="$emit('click')"
      />
      <div
        v-else
        :data-testid="'user-quality-' + mode"
      >{{ mode }} {{ stats?.p50_ttft_ms ?? '' }} {{ stats?.success_rate ?? '' }}</div>
    `
  }
}))
vi.mock('@/components/user/UserConcurrencyCell.vue', () => ({
  default: {
    props: ['current', 'max'],
    template: '<div data-testid="user-concurrency-cell">{{ current }}/{{ max }}</div>'
  }
}))
vi.mock('@/components/user/UserBurnRateCell.vue', () => ({
  default: {
    props: ['stats', 'unit'],
    template:
      '<div data-testid="user-burn-rate-cell">${{ Number(stats?.burn_rate_per_hour ?? 0).toFixed(2) }}/h</div>'
  }
}))
vi.mock('@/components/common/GroupBadge.vue', () => ({
  default: {
    props: ['name'],
    template: '<span>{{ name }}</span>'
  }
}))
vi.mock('@/components/account/AccountTodayStatsCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountGroupsCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountUsageCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountInlineNumberCell.vue', () => ({
  default: {
    props: ['modelValue'],
    template: '<div>{{ modelValue }}</div>'
  }
}))
vi.mock('@/components/account/AccountCapacityCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/CapacityBadge.vue', () => ({
  default: {
    props: ['current', 'max'],
    template: '<div data-testid="smart-schedule-pair-badge">{{ current }}/{{ max }}</div>'
  }
}))
vi.mock('@/components/admin/smart-schedule/SmartSchedulePoolFilters.vue', () => ({
  default: {
    props: ['filters'],
    emits: ['update:filters'],
    template: `
      <div data-testid="smart-schedule-pool-filters">
        <button
          data-testid="smart-schedule-filter-stopped"
          @click="$emit('update:filters', { ...filters, admission: 'stopped' })"
        />
      </div>
    `
  }
}))
vi.mock('@/components/admin/smart-schedule/SmartSchedulePoolBulkBar.vue', () => ({
  default: {
    props: ['selectedIds', 'filteredCount', 'bulkCap'],
    emits: ['select-page', 'select-matching', 'clear', 'apply-cap', 'apply-cap-all', 'remove', 'update:bulkCap'],
    template: `
      <div data-testid="smart-schedule-bulk-region">
        <div data-testid="smart-schedule-pool-bulk-bar">
          <button data-testid="smart-schedule-select-page" @click="$emit('select-page')" />
          <button data-testid="smart-schedule-select-matching" @click="$emit('select-matching')" />
          <button data-testid="smart-schedule-batch-remove" @click="$emit('remove')" />
        </div>
      </div>
    `
  }
}))
vi.mock('@/components/admin/smart-schedule/SmartScheduleAddAccountDialog.vue', () => ({
  default: {
    props: ['show', 'accounts', 'platform'],
    emits: ['close', 'add'],
    template: `
      <div v-if="show" data-testid="smart-schedule-add-dialog">
        <button
          data-testid="smart-schedule-add-dialog-all"
          @click="$emit('add', (accounts || []).map((item) => item.id))"
        />
      </div>
    `
  }
}))
vi.mock('@/components/account/AccountStabilityDialog.vue', () => ({
  default: {
    props: ['show', 'account'],
    template: '<div v-if="show" data-testid="account-stability-dialog" />'
  }
}))
vi.mock('@/components/account', () => ({
  EditAccountModal: { template: '<div />' },
  TempUnschedStatusModal: { template: '<div />' }
}))
vi.mock('@/components/admin/account/AccountActionMenu.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/account/ReAuthAccountModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/account/UpdateRefreshTokenModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/account/AccountTestModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/account/AccountStatsModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/account/ScheduledTestsPanel.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/usage/UsageErrorInspectDialog.vue', () => ({ default: { template: '<div />' } }))

import UserSmartScheduleView from '../UserSmartScheduleView.vue'

function emptyPlatform() {
  return {
    enabled: false,
    quality_max_p50_ttft_ms: null,
    quality_min_success_rate: null,
    quality_min_success_samples: null,
    quality_min_ttft_samples: null,
    quality_condition: null,
    cooldown_minutes: 15,
    accounts: []
  }
}

function makeView() {
  return {
    user_id: 99,
    platforms: {
      anthropic: emptyPlatform(),
      openai: emptyPlatform(),
      gemini: emptyPlatform(),
      antigravity: emptyPlatform(),
      grok: emptyPlatform()
    }
  }
}

async function mountPage() {
  const w = mount(UserSmartScheduleView)
  await flushPromises()
  await flushPromises()
  return w
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  localStorage.removeItem('smart-schedule-pool-hidden-columns')
  localStorage.removeItem('smart-schedule-pool-column-layout')
  localStorage.removeItem('smart-schedule-auto-refresh')
  localStorage.removeItem('smart-schedule-pool-sort')
  apiMocks.getById.mockResolvedValue({
    id: 99,
    email: 'u@example.com',
    username: 'u',
    role: 'user',
    status: 'active',
    balance: 12.5,
    concurrency: 8,
    allowed_groups: []
  })
  apiMocks.getSmartSchedule.mockResolvedValue(makeView())
  apiMocks.updateSmartSchedule.mockResolvedValue(makeView())
  apiMocks.updateSmartScheduleSortOrder.mockResolvedValue(makeView())
  apiMocks.copySmartSchedule.mockResolvedValue(makeView())
  apiMocks.listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
  apiMocks.listUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 5, pages: 0 })
  apiMocks.getUserBatchQualityStats.mockResolvedValue({ stats: {} })
  apiMocks.getBatchUsersUsage.mockResolvedValue({ stats: {} })
  apiMocks.getBatchUsersBurnRate.mockResolvedValue({ stats: {} })
  apiMocks.getBatchQualityStats.mockResolvedValue({ stats: {} })
  apiMocks.getBatchTodayStats.mockResolvedValue({ stats: {} })
  apiMocks.updateAccount.mockReset()
  apiMocks.updateAccount.mockResolvedValue({})
  apiMocks.moveAccountToTop.mockReset()
  apiMocks.resumeSmartSchedule.mockReset()
  apiMocks.resumeSmartSchedule.mockResolvedValue({ account_id: 11, user_id: 99 })
  tableMocks.setSort.mockReset()
})

describe('UserSmartScheduleView', () => {
  it('loads smart schedule for the route user', async () => {
    await mountPage()
    expect(apiMocks.getSmartSchedule).toHaveBeenCalledWith(99)
    expect(apiMocks.getById).toHaveBeenCalledWith(99)
  })

  it('uses a compact header, two cards, and a full-width pool table', async () => {
    const w = await mountPage()
    const header = w.get('[data-testid="smart-schedule-page-header"]')
    expect(header.get('[data-testid="smart-schedule-back"]').exists()).toBe(true)
    expect(header.text()).toContain('admin.users.smartSchedule.pageDescription')
    expect(header.text()).not.toContain('admin.users.smartSchedule.title')
    expect(header.text()).not.toContain('admin.users.smartSchedule.subtitle')
    expect(w.find('h1').exists()).toBe(false)

    const layout = w.get('[data-testid="smart-schedule-layout"]')
    expect(layout.classes()).toContain('grid')
    expect(layout.classes()).toContain('items-stretch')
    expect(layout.classes()).toContain('lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]')
    expect(layout.classes().join(' ')).not.toContain('lg:items-start')
    expect(layout.get('[data-testid="smart-schedule-user-panel"]').exists()).toBe(true)
    expect(layout.get('[data-testid="smart-schedule-pool-panel"]').exists()).toBe(true)
    expect(layout.find('[data-testid="smart-schedule-pool-table"]').exists()).toBe(false)
    expect(layout.find('[data-testid="smart-schedule-bulk-region"]').exists()).toBe(false)

    const userPanel = w.get('[data-testid="smart-schedule-user-panel"]')
    expect(userPanel.classes().join(' ')).not.toMatch(/lg:w-\[26rem\]|xl:w-\[28rem\]/)
    expect(userPanel.text()).toContain('u@example.com')
    const toolbar = userPanel.get('[data-testid="smart-schedule-threshold-toolbar"]')
    expect(toolbar.get('[data-testid="smart-schedule-enable-card"]').exists()).toBe(true)
    expect(toolbar.get('[data-testid="smart-schedule-enabled"]').exists()).toBe(true)
    expect(toolbar.get('[data-testid="smart-schedule-tabs"]').exists()).toBe(true)
    const thresholdGrid = userPanel.get('[data-testid="smart-schedule-threshold-grid"]')
    expect(thresholdGrid.classes()).toContain('grid-cols-2')
    expect(thresholdGrid.classes()).toContain('lg:grid-cols-3')
    expect(thresholdGrid.get('[data-testid="smart-schedule-p50"]').exists()).toBe(true)
    expect(thresholdGrid.get('[data-testid="smart-schedule-success"]').exists()).toBe(true)
    expect(thresholdGrid.get('[data-testid="smart-schedule-cooldown"]').exists()).toBe(true)

    expect(w.get('[data-testid="smart-schedule-pool-table-region"]').get('[data-testid="smart-schedule-pool-table"]').exists()).toBe(true)
  })

  it('renders all platform tabs', async () => {
    const w = await mountPage()
    const html = w.html()
    expect(html).toContain('smart-schedule-tab-anthropic')
    expect(html).toContain('smart-schedule-tab-openai')
    expect(html).toContain('smart-schedule-tab-gemini')
    expect(html).toContain('smart-schedule-tab-antigravity')
    expect(html).toContain('smart-schedule-tab-grok')
  })

  it('does not save enabled empty pool', async () => {
    const w = await mountPage()
    await w.get('[data-testid="smart-schedule-enabled"]').trigger('click')
    await flushPromises()
    expect(apiMocks.updateSmartSchedule).not.toHaveBeenCalled()
    expect(w.get('[data-testid="smart-schedule-empty-error"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-enabled"]').attributes('aria-checked')).toBe('false')
  })

  it('enables a platform immediately when the pool has members', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: false,
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: 2 }]
        }
      }
    })
    apiMocks.updateSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: 2 }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'acc-11', platform: 'anthropic', status: 'active' }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    const w = await mountPage()
    await w.get('[data-testid="smart-schedule-enabled"]').trigger('click')
    await flushPromises()
    expect(apiMocks.updateSmartSchedule).toHaveBeenCalledWith(
      99,
      'anthropic',
      expect.objectContaining({ enabled: true, accounts: [expect.objectContaining({ account_id: 11 })] })
    )
  })

  it('copies settings from another platform', async () => {
    const w = await mountPage()
    await w.get('[data-testid="smart-schedule-copy-from"]').setValue('openai')
    await w.get('[data-testid="smart-schedule-copy"]').trigger('click')
    await flushPromises()
    expect(apiMocks.copySmartSchedule).toHaveBeenCalledWith(99, 'anthropic', 'openai')
  })

  it('disables the platform when the last pool member is removed', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: 2 }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'acc-11', platform: 'anthropic', status: 'active' }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    const w = await mountPage()
    await w.get('[data-testid="smart-schedule-remove"]').trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="smart-schedule-enabled"]').attributes('aria-checked')).toBe('false')
  })

  it('adds currently scheduling API accounts in one click', async () => {
    const candidates = [
      { id: 1, name: 'api-live', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true },
      { id: 2, name: 'oauth-live', platform: 'anthropic', type: 'oauth', status: 'active', schedulable: true },
      { id: 3, name: 'api-paused', platform: 'anthropic', type: 'apikey', status: 'inactive', schedulable: true }
    ]
    apiMocks.listAccounts.mockImplementation((_page: number, _size: number, filters?: { ids?: string }) => {
      if (filters?.ids) {
        const ids = filters.ids.split(',').map(Number)
        return Promise.resolve({
          items: candidates.filter((item) => ids.includes(item.id)),
          total: ids.length,
          page: 1,
          page_size: ids.length,
          pages: 1
        })
      }
      return Promise.resolve({ items: candidates, total: candidates.length, page: 1, page_size: 1000, pages: 1 })
    })
    const w = await mountPage()
    expect(w.get('[data-testid="smart-schedule-add-api"]').attributes('disabled')).toBeUndefined()
    await w.get('[data-testid="smart-schedule-add-api"]').trigger('click')
    await flushPromises()
    await flushPromises()
    expect(w.text()).toContain('api-live')
    expect(w.text()).not.toContain('oauth-live')
    expect(w.text()).not.toContain('api-paused')
  })

  it('adds all currently scheduling accounts in one click', async () => {
    const candidates = [
      { id: 1, name: 'api-live', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true },
      { id: 2, name: 'oauth-live', platform: 'anthropic', type: 'oauth', status: 'active', schedulable: true }
    ]
    apiMocks.listAccounts.mockImplementation((_page: number, _size: number, filters?: { ids?: string }) => {
      if (filters?.ids) {
        const ids = filters.ids.split(',').map(Number)
        return Promise.resolve({
          items: candidates.filter((item) => ids.includes(item.id)),
          total: ids.length,
          page: 1,
          page_size: ids.length,
          pages: 1
        })
      }
      return Promise.resolve({ items: candidates, total: candidates.length, page: 1, page_size: 1000, pages: 1 })
    })
    const w = await mountPage()
    expect(w.get('[data-testid="smart-schedule-add-all"]').attributes('disabled')).toBeUndefined()
    await w.get('[data-testid="smart-schedule-add-all"]').trigger('click')
    await flushPromises()
    await flushPromises()
    expect(w.text()).toContain('api-live')
    expect(w.text()).toContain('oauth-live')
  })

  it('filters the single-add dropdown by typed account name', async () => {
    const candidates = [
      { id: 21, name: 'alpha-bot', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true },
      { id: 22, name: 'beta-bot', platform: 'anthropic', type: 'oauth', status: 'active', schedulable: true }
    ]
    apiMocks.listAccounts.mockImplementation((_page: number, _size: number, filters?: { ids?: string }) => {
      if (filters?.ids) {
        const ids = filters.ids.split(',').map(Number)
        return Promise.resolve({
          items: candidates.filter((item) => ids.includes(item.id)),
          total: ids.length,
          page: 1,
          page_size: ids.length,
          pages: 1
        })
      }
      return Promise.resolve({ items: candidates, total: 2, page: 1, page_size: 1000, pages: 1 })
    })
    const w = await mountPage()
    await w.get('[data-testid="smart-schedule-add-select"]').trigger('focus')
    await w.get('[data-testid="smart-schedule-add-select"]').setValue('alpha')
    await w.get('[data-testid="smart-schedule-add-select"]').trigger('input')
    await flushPromises()
    await flushPromises()
    const dropdown = w.get('[data-testid="smart-schedule-add-dropdown"]')
    expect(dropdown.text()).toContain('alpha-bot (#21)')
    expect(dropdown.text()).not.toContain('beta-bot')
    await w.get('[data-testid="smart-schedule-add-option-21"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('alpha-bot')
  })

  it('reuses account-list sortable columns and opens the quality dialog', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: 2, current_concurrency: 1 }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'acc-11', platform: 'anthropic', type: 'apikey', status: 'active', concurrency: 4 }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    const w = await mountPage()
    const headers = w.get('[data-testid="smart-schedule-pool-headers"]')
    expect(headers.get('[data-column="name"]').attributes('data-sortable')).toBe('true')
    expect(headers.get('[data-column="concurrency"]').attributes('data-sortable')).toBe('true')
    expect(headers.get('[data-column="pair_cap"]').attributes('data-sortable')).toBe('true')
    expect(headers.get('[data-column="status"]').attributes('data-sortable')).toBe('true')
    expect(headers.get('[data-column="quality_ttft"]').attributes('data-sortable')).toBe('false')
    expect(w.get('[data-testid="account-open-stability"]').exists()).toBe(true)
    await w.get('[data-testid="account-quality-cell-button"]').trigger('click')
    expect(w.get('[data-testid="account-stability-dialog"]').exists()).toBe(true)
  })

  it('can hide a pool column from column settings', async () => {
    const w = await mountPage()
    await w.get('[data-testid="smart-schedule-column-settings"]').trigger('click')
    const qualityRow = w
      .findAll('[data-testid="smart-schedule-column-settings-row"]')
      .find((row) => row.attributes('data-column-key') === 'quality_ttft')
    expect(qualityRow).toBeTruthy()
    await qualityRow!.get('button').trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="quality_ttft"]').exists()).toBe(false)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="name"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="actions"]').exists()).toBe(true)
  })

  it('opens the only enabled platform instead of always anthropic', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 16,
      default_platform: 'openai',
      platforms: {
        ...makeView().platforms,
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 21, platform: 'openai', max_concurrency: null }]
        }
      }
    })
    apiMocks.getById.mockResolvedValue({
      id: 16,
      email: 'zuoge85@example.com',
      username: 'zuoge85',
      role: 'user',
      status: 'active',
      balance: 12.5,
      concurrency: 20
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 21, name: 'oa-1', platform: 'openai', type: 'oauth', status: 'active', schedulable: true }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    const w = await mountPage()
    expect(w.get('[data-testid="smart-schedule-tab-openai"]').attributes('data-active')).toBe('true')
    expect(w.get('[data-testid="smart-schedule-tab-anthropic"]').attributes('data-active')).toBe('false')
    expect(w.get('[data-testid="smart-schedule-user-row"]').text()).toContain('12.5')
    const listCalls = apiMocks.listAccounts.mock.calls as Array<
      [number, number, { ids?: string; lite?: string; platform?: string }]
    >
    const poolCalls = listCalls.filter((call) => Boolean(call[2]?.ids))
    const candidateCalls = listCalls.filter((call) => !call[2]?.ids)
    expect(poolCalls).toHaveLength(1)
    expect(poolCalls[0]?.[2]).toMatchObject({ platform: 'openai', ids: '21', lite: '1' })
    expect(candidateCalls).toHaveLength(0)
  })

  it('renders the users-list row including TTFT and success rate', async () => {
    apiMocks.getUserBatchQualityStats.mockResolvedValue({
      stats: {
        '99': {
          window_seconds: 900,
          success_count: 20,
          error_count: 1,
          success_rate: 0.97,
          avg_ttft_ms: 400,
          p50_ttft_ms: 320,
          p95_ttft_ms: 800,
          max_ttft_ms: 1100,
          ttft_samples: 12
        }
      }
    })
    apiMocks.getBatchUsersUsage.mockResolvedValue({
      stats: {
        '99': { user_id: 99, today_actual_cost: 1.2345, total_actual_cost: 9.8765 }
      }
    })
    apiMocks.getBatchUsersBurnRate.mockResolvedValue({
      stats: {
        '99': {
          user_id: 99,
          recent_5m_actual_cost: 0.1,
          burn_rate_per_hour: 1.2,
          window_seconds: 300
        }
      }
    })
    const w = await mountPage()
    expect(apiMocks.getUserBatchQualityStats).toHaveBeenCalledWith([99])
    expect(apiMocks.getBatchUsersUsage).toHaveBeenCalledWith([99])
    expect(apiMocks.getBatchUsersBurnRate).toHaveBeenCalledWith([99])
    const row = w.get('[data-testid="smart-schedule-user-row"]')
    const headers = w.get('[data-testid="smart-schedule-user-headers"]')
    expect(headers.find('[data-column="username"]').exists()).toBe(false)
    expect(headers.get('[data-column="burn_rate"]').exists()).toBe(true)
    expect(headers.get('[data-column="quality_ttft"]').exists()).toBe(true)
    expect(headers.get('[data-column="quality_success_rate"]').exists()).toBe(true)
    expect(headers.get('[data-column="usage"]').exists()).toBe(true)
    expect(headers.get('[data-column="concurrency"]').exists()).toBe(true)
    expect(row.get('[data-testid="user-quality-ttft"]').text()).toContain('320')
    expect(row.get('[data-testid="user-quality-success_rate"]').text()).toContain('0.97')
    expect(row.get('[data-testid="user-concurrency-cell"]').text()).toContain('8')
    expect(row.get('[data-testid="user-burn-rate-cell"]').text()).toContain('$1.20/h')
    expect(row.text()).toContain('1.2345')
  })

  it('refreshes header burn rate through the extras loader', async () => {
    apiMocks.getBatchUsersBurnRate.mockResolvedValue({
      stats: {
        '99': {
          user_id: 99,
          recent_5m_actual_cost: 0.25,
          burn_rate_per_hour: 3,
          window_seconds: 300
        }
      }
    })
    const w = await mountPage()
    expect(apiMocks.getBatchUsersBurnRate).toHaveBeenCalledWith([99])
    await w.get('[data-testid="smart-schedule-refresh"]').trigger('click')
    await flushPromises()
    expect(apiMocks.getBatchUsersBurnRate.mock.calls.length).toBeGreaterThanOrEqual(2)
    expect(w.get('[data-testid="user-burn-rate-cell"]').text()).toContain('$3.00/h')
    expect(w.get('[data-testid="smart-schedule-layout"]').exists()).toBe(true)
    expect(w.find('[data-testid="smart-schedule-page-loading"]').exists()).toBe(false)
  })

  it('does not use account concurrency as the uncapped pair denominator', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: null, current_concurrency: 0 }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'acc-11', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true, concurrency: 99 }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    const w = await mountPage()
    const badge = w.get('[data-testid="smart-schedule-pair-badge"]')
    expect(badge.text()).toBe('0/999')
    expect(badge.text()).not.toBe('0/99')
    expect(w.find('[data-testid="smart-schedule-pair-uncapped"]').exists()).toBe(false)
  })

  it('shows uncapped pair occupancy with display denominator 999', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: null, current_concurrency: 3 }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'acc-11', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true, concurrency: 8 }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    const w = await mountPage()
    expect(w.get('[data-testid="smart-schedule-pair-badge"]').text()).toBe('3/999')
    expect(w.text()).not.toContain('3/8')
  })

  it('keeps the real pair cap as the badge max when capped', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: 2, current_concurrency: 1 }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'acc-11', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true, concurrency: 8 }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    const w = await mountPage()
    expect(w.get('[data-testid="smart-schedule-pair-badge"]').text()).toBe('1/2')
    expect(w.text()).not.toContain('1/999')
    expect(w.text()).not.toContain('1/8')
  })

  it('batch-removes currently filtered stopped-scheduling accounts', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 11, platform: 'anthropic', max_concurrency: null },
            { account_id: 12, platform: 'anthropic', max_concurrency: null }
          ]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [
        { id: 11, name: 'live-acc', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true },
        { id: 12, name: 'stopped-acc', platform: 'anthropic', type: 'oauth', status: 'active', schedulable: false }
      ],
      total: 2,
      page: 1,
      page_size: 2,
      pages: 1
    })
    const w = await mountPage()
    expect(w.text()).toContain('live-acc')
    expect(w.text()).toContain('stopped-acc')
    await w.get('[data-testid="smart-schedule-filter-stopped"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('stopped-acc')
    expect(w.text()).not.toContain('live-acc')
    await w.get('[data-testid="smart-schedule-select-matching"]').trigger('click')
    await w.get('[data-testid="smart-schedule-batch-remove"]').trigger('click')
    await flushPromises()
    expect(w.text()).not.toContain('stopped-acc')
    expect(w.get('[data-testid="smart-schedule-dirty-banner"]').exists()).toBe(true)
  })

  it('renders refresh controls and an admission column', async () => {
    const w = await mountPage()
    const addRegion = w.get('[data-testid="smart-schedule-add-region"]')
    expect(addRegion.get('[data-testid="smart-schedule-refresh"]').exists()).toBe(true)
    expect(addRegion.get('[data-testid="smart-schedule-auto-refresh"]').exists()).toBe(true)
    expect(addRegion.get('[data-testid="smart-schedule-auto-sort"]').exists()).toBe(true)
    expect(addRegion.get('[data-testid="smart-schedule-add-ops"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="smart-schedule-refresh"]')).toHaveLength(1)
    expect(w.get('[data-testid="smart-schedule-enable-card"]').find('[data-testid="smart-schedule-refresh"]').exists()).toBe(false)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="admission"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="sort_order"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="priority"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="select"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="priority"]').text()).toBe(
      'admin.users.smartSchedule.accountPriority'
    )
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="sort_order"]').text()).toBe(
      'admin.users.smartSchedule.poolSortOrder'
    )
  })

  it('shows live account priority in the priority column, not pool sort_order', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        ...makeView().platforms,
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 21, platform: 'openai', max_concurrency: null, sort_order: 1, priority: 1 },
            { account_id: 22, platform: 'openai', max_concurrency: null, sort_order: 2, priority: 2 }
          ]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [
        { id: 21, name: 'oa-1', platform: 'openai', type: 'oauth', status: 'active', priority: 80 },
        { id: 22, name: 'oa-2', platform: 'openai', type: 'oauth', status: 'active', priority: 3 }
      ],
      total: 2,
      page: 1,
      page_size: 2,
      pages: 1
    })
    const w = await mountPage()
    expect(w.get('[data-testid="smart-schedule-pool-sort-order-21"]').text()).toBe('1')
    expect(w.get('[data-testid="smart-schedule-pool-priority-21"]').text()).toBe('80')
    expect(w.get('[data-testid="smart-schedule-pool-sort-order-22"]').text()).toBe('2')
    expect(w.get('[data-testid="smart-schedule-pool-priority-22"]').text()).toBe('3')
  })

  it('keeps add, pool-filter, and bulk regions distinct', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: null }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'acc-11', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    const w = await mountPage()
    const poolPanel = w.get('[data-testid="smart-schedule-pool-panel"]')
    expect(poolPanel.get('[data-testid="smart-schedule-add-region"]').exists()).toBe(true)
    expect(poolPanel.get('[data-testid="smart-schedule-add-cluster"]').exists()).toBe(true)
    expect(poolPanel.get('[data-testid="smart-schedule-add-ops"]').exists()).toBe(true)
    expect(poolPanel.get('[data-testid="smart-schedule-filtered-add"]').exists()).toBe(true)
    expect(poolPanel.get('[data-testid="smart-schedule-pool-filters"]').exists()).toBe(true)
    expect(poolPanel.find('[data-testid="smart-schedule-bulk-region"]').exists()).toBe(false)
    expect(w.get('[data-testid="smart-schedule-pool-table-region"]').get('[data-testid="smart-schedule-bulk-region"]').exists()).toBe(true)
    expect(w.find('[data-testid="smart-schedule-add-dialog"]').exists()).toBe(false)
  })

  it('adds matching candidates from the filtered-add dialog without writing until save', async () => {
    const candidates = [
      { id: 31, name: 'dialog-api', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true },
      { id: 32, name: 'dialog-oauth', platform: 'anthropic', type: 'oauth', status: 'active', schedulable: true }
    ]
    apiMocks.listAccounts.mockImplementation((_page: number, _size: number, filters?: { ids?: string }) => {
      if (filters?.ids) {
        const ids = filters.ids.split(',').map(Number)
        return Promise.resolve({
          items: candidates.filter((item) => ids.includes(item.id)),
          total: ids.length,
          page: 1,
          page_size: ids.length,
          pages: 1
        })
      }
      return Promise.resolve({ items: candidates, total: 2, page: 1, page_size: 1000, pages: 1 })
    })
    const w = await mountPage()
    await w.get('[data-testid="smart-schedule-filtered-add"]').trigger('click')
    await flushPromises()
    await flushPromises()
    expect(w.get('[data-testid="smart-schedule-add-dialog"]').exists()).toBe(true)
    await w.get('[data-testid="smart-schedule-add-dialog-all"]').trigger('click')
    await flushPromises()
    await flushPromises()
    expect(w.text()).toContain('dialog-api')
    expect(w.text()).toContain('dialog-oauth')
    expect(w.get('[data-testid="smart-schedule-dirty-banner"]').exists()).toBe(true)
    expect(apiMocks.updateSmartSchedule).not.toHaveBeenCalled()
  })

  it('shows auto-sort and calls the pool sort helper', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 11, platform: 'anthropic', max_concurrency: null },
            { account_id: 12, platform: 'anthropic', max_concurrency: 1 }
          ]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [
        { id: 11, name: 'live-acc', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true, concurrency: 2, priority: 8 },
        { id: 12, name: 'stopped-acc', platform: 'anthropic', type: 'oauth', status: 'active', schedulable: false, concurrency: 9, priority: 1 }
      ],
      total: 2,
      page: 1,
      page_size: 2,
      pages: 1
    })
    const w = await mountPage()
    const button = w.get('[data-testid="smart-schedule-auto-sort"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')
    await flushPromises()
    expect(autoSortMocks.sortSmartSchedulePoolMembers).toHaveBeenCalled()
    const firstCall = autoSortMocks.sortSmartSchedulePoolMembers.mock.calls[0]?.[0] as Array<{
      id: number
      upstreamRate: number
      lastUsedAt?: string | null
    }>
    expect(firstCall.map((row) => row.id).sort((a, b) => a - b)).toEqual([11, 12])
    expect(firstCall.every((row) => typeof row.upstreamRate === 'number')).toBe(true)
    expect(firstCall.every((row) => !('lastUsedAt' in row))).toBe(true)
    expect(apiMocks.updateSmartScheduleSortOrder).toHaveBeenCalled()
    const payload = apiMocks.updateSmartScheduleSortOrder.mock.calls[0]?.[2] as {
      accounts: Array<{ account_id: number; sort_order: number; priority?: number }>
    }
    expect(payload.accounts.map((row) => row.sort_order)).toEqual([1, 2])
    expect(payload.accounts.every((row) => Object.keys(row).sort().join() === 'account_id,sort_order')).toBe(true)
    expect(apiMocks.updateAccount).not.toHaveBeenCalled()
    expect(tableMocks.setSort).toHaveBeenCalledWith('sort_order', 'asc')
  })

  it('move-to-top writes pool sort_order only and does not change account.priority', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 11, platform: 'anthropic', max_concurrency: null },
            { account_id: 12, platform: 'anthropic', max_concurrency: 1 }
          ]
        }
      }
    })
    const first = {
      id: 11,
      name: 'first-acc',
      platform: 'anthropic',
      type: 'apikey',
      status: 'active',
      schedulable: true,
      concurrency: 2,
      priority: 0
    }
    const later = {
      id: 12,
      name: 'later-acc',
      platform: 'anthropic',
      type: 'oauth',
      status: 'active',
      schedulable: true,
      concurrency: 1,
      priority: 3
    }
    apiMocks.listAccounts.mockResolvedValue({
      items: [first, later],
      total: 2,
      page: 1,
      page_size: 2,
      pages: 1
    })
    apiMocks.updateSmartScheduleSortOrder.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 12, platform: 'anthropic', max_concurrency: 1, sort_order: 1 },
            { account_id: 11, platform: 'anthropic', max_concurrency: null, sort_order: 2 }
          ]
        }
      }
    })
    const w = await mountPage()
    const buttons = w.findAll('[data-testid="account-move-to-top"]')
    expect(buttons).toHaveLength(2)
    await buttons[1].trigger('click')
    await flushPromises()
    expect(apiMocks.moveAccountToTop).not.toHaveBeenCalled()
    expect(apiMocks.updateAccount).not.toHaveBeenCalled()
    expect(apiMocks.updateSmartScheduleSortOrder).toHaveBeenCalledWith(
      99,
      'anthropic',
      {
        accounts: [
          { account_id: 12, sort_order: 1 },
          { account_id: 11, sort_order: 2 }
        ]
      }
    )
    expect(tableMocks.setSort).toHaveBeenCalledWith('sort_order', 'asc')
  })

  it('labels a saved-gate miss without cooldown as will-cool, not a quality lock', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          quality_max_p50_ttft_ms: 200,
          quality_min_success_rate: 0.9,
          quality_min_success_samples: 1,
          quality_min_ttft_samples: 1,
          quality_condition: 'or',
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: null }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'live-acc', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    apiMocks.getBatchQualityStats.mockResolvedValue({
      stats: {
        '11': {
          window_seconds: 900,
          success_count: 0,
          error_count: 8,
          success_rate: 0,
          p50_ttft_ms: 900,
          ttft_samples: 8
        }
      }
    })
    const w = await mountPage()
    const cell = w.get('[data-testid="smart-schedule-admission"]')
    expect(cell.attributes('data-admission')).toBe('will_cool')
    expect(cell.text()).toContain('admin.users.smartSchedule.admissionWillCool')
    expect(cell.text()).not.toContain('admin.users.smartSchedule.admissionQualityBlocked')
    expect(w.get('[data-testid="smart-schedule-resume-will-cool"]').exists()).toBe(true)
  })

  it('after resume, shows resumed instead of will-cool', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          quality_max_p50_ttft_ms: 200,
          quality_min_success_rate: 0.9,
          quality_min_success_samples: 1,
          quality_min_ttft_samples: 1,
          quality_condition: 'or',
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: null }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'live-acc', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    apiMocks.getBatchQualityStats.mockResolvedValue({
      stats: {
        '11': {
          window_seconds: 900,
          success_count: 0,
          error_count: 8,
          success_rate: 0,
          p50_ttft_ms: 900,
          ttft_samples: 8
        }
      }
    })
    const w = await mountPage()
    await w.get('[data-testid="smart-schedule-resume-will-cool"]').trigger('click')
    await flushPromises()
    expect(apiMocks.resumeSmartSchedule).toHaveBeenCalledWith(11, 99)
    expect(w.get('[data-testid="smart-schedule-admission"]').attributes('data-admission')).toBe('resumed')
    expect(w.get('[data-testid="smart-schedule-admission"]').text()).toContain(
      'admin.users.smartSchedule.admissionResumed'
    )
  })

  it('labels a tighter unsaved draft as preview when the saved gate still passes', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      platforms: {
        ...makeView().platforms,
        anthropic: {
          ...emptyPlatform(),
          enabled: true,
          quality_max_p50_ttft_ms: 2000,
          quality_min_success_rate: 0.1,
          quality_min_success_samples: 1,
          quality_min_ttft_samples: 1,
          quality_condition: 'or',
          accounts: [{ account_id: 11, platform: 'anthropic', max_concurrency: null }]
        }
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 11, name: 'live-acc', platform: 'anthropic', type: 'apikey', status: 'active', schedulable: true }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    apiMocks.getBatchQualityStats.mockResolvedValue({
      stats: {
        '11': {
          window_seconds: 900,
          success_count: 8,
          error_count: 2,
          success_rate: 0.8,
          p50_ttft_ms: 400,
          ttft_samples: 10
        }
      }
    })
    const w = await mountPage()
    expect(w.get('[data-testid="smart-schedule-admission"]').attributes('data-admission')).toBe('selectable')
    const p50 = w.get('[data-testid="smart-schedule-p50"]')
    await p50.setValue(100)
    await flushPromises()
    expect(w.get('[data-testid="smart-schedule-admission"]').attributes('data-admission')).toBe('unsaved_preview')
    expect(w.get('[data-testid="smart-schedule-admission"]').text()).toContain(
      'admin.users.smartSchedule.admissionUnsavedPreview'
    )
    expect(w.get('[data-testid="smart-schedule-resume-preview"]').attributes('disabled')).toBeDefined()
  })

  it('does not fetch candidates on first paint', async () => {
    await mountPage()
    const listCalls = apiMocks.listAccounts.mock.calls as Array<[number, number, { ids?: string }]>
    expect(listCalls.every((call) => Boolean(call[2]?.ids) || call.length < 3)).toBe(true)
    expect(listCalls.filter((call) => !call[2]?.ids)).toHaveLength(0)
  })
})
