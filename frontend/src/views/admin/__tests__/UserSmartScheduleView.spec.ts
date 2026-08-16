import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const apiMocks = vi.hoisted(() => ({
  getById: vi.fn(),
  listUsers: vi.fn(),
  getUserBatchQualityStats: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  getSmartSchedule: vi.fn(),
  updateSmartSchedule: vi.fn(),
  copySmartSchedule: vi.fn(),
  listAccounts: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getQualityHardCloseSettings: vi.fn(),
  updateQualityHardCloseSettings: vi.fn()
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
      copySmartSchedule: apiMocks.copySmartSchedule
    },
    dashboard: {
      getBatchUsersUsage: apiMocks.getBatchUsersUsage
    },
    accounts: {
      list: apiMocks.listAccounts,
      getBatchQualityStats: apiMocks.getBatchQualityStats,
      getBatchTodayStats: apiMocks.getBatchTodayStats,
      update: vi.fn().mockResolvedValue({}),
      setSchedulable: vi.fn(),
      resumeSmartSchedule: vi.fn(),
      moveAccountToTop: vi.fn()
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
          <slot name="cell-email" :row="row" :value="row.email" />
          <slot name="cell-balance" :row="row" :value="row.balance" />
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
vi.mock('@/components/common/GroupBadge.vue', () => ({
  default: {
    props: ['name'],
    template: '<span>{{ name }}</span>'
  }
}))
vi.mock('@/components/account/AccountTodayStatsCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountGroupsCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountUsageCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountInlineNumberCell.vue', () => ({ default: { template: '<div />' } }))
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
  apiMocks.copySmartSchedule.mockResolvedValue(makeView())
  apiMocks.listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
  apiMocks.listUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 5, pages: 0 })
  apiMocks.getUserBatchQualityStats.mockResolvedValue({ stats: {} })
  apiMocks.getBatchUsersUsage.mockResolvedValue({ stats: {} })
  apiMocks.getBatchQualityStats.mockResolvedValue({ stats: {} })
  apiMocks.getBatchTodayStats.mockResolvedValue({ stats: {} })
})

describe('UserSmartScheduleView', () => {
  it('loads smart schedule for the route user', async () => {
    await mountPage()
    expect(apiMocks.getSmartSchedule).toHaveBeenCalledWith(99)
    expect(apiMocks.getById).toHaveBeenCalledWith(99)
  })

  it('uses a left user panel and right pool panel', async () => {
    const w = await mountPage()
    expect(w.get('[data-testid="smart-schedule-user-panel"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-panel"]').exists()).toBe(true)
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
    await w.get('[data-testid="smart-schedule-add-api"]').trigger('click')
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
    await w.get('[data-testid="smart-schedule-add-all"]').trigger('click')
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
    await w.get('[data-testid="smart-schedule-add-select"]').setValue('alpha')
    await w.get('[data-testid="smart-schedule-add-select"]').trigger('input')
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
    const w = await mountPage()
    expect(apiMocks.getUserBatchQualityStats).toHaveBeenCalledWith([99])
    expect(apiMocks.getBatchUsersUsage).toHaveBeenCalledWith([99])
    const row = w.get('[data-testid="smart-schedule-user-row"]')
    const headers = w.get('[data-testid="smart-schedule-user-headers"]')
    expect(headers.get('[data-column="quality_ttft"]').exists()).toBe(true)
    expect(headers.get('[data-column="quality_success_rate"]').exists()).toBe(true)
    expect(headers.get('[data-column="usage"]').exists()).toBe(true)
    expect(headers.get('[data-column="concurrency"]').exists()).toBe(true)
    expect(row.get('[data-testid="user-quality-ttft"]').text()).toContain('320')
    expect(row.get('[data-testid="user-quality-success_rate"]').text()).toContain('0.97')
    expect(row.get('[data-testid="user-concurrency-cell"]').text()).toContain('8')
    expect(row.text()).toContain('1.2345')
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
    expect(w.get('[data-testid="smart-schedule-pair-uncapped"]').exists()).toBe(true)
    expect(w.text()).not.toContain('0/99')
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
    expect(w.get('[data-testid="smart-schedule-refresh"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-auto-refresh"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="admission"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-headers"]').find('[data-column="select"]').exists()).toBe(true)
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
    expect(w.get('[data-testid="smart-schedule-add-region"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-filtered-add"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pool-filters"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-bulk-region"]').exists()).toBe(true)
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
    expect(w.get('[data-testid="smart-schedule-add-dialog"]').exists()).toBe(true)
    await w.get('[data-testid="smart-schedule-add-dialog-all"]').trigger('click')
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
    const firstCall = autoSortMocks.sortSmartSchedulePoolMembers.mock.calls[0]?.[0] as Array<{ id: number }>
    expect(firstCall.map((row) => row.id).sort((a, b) => a - b)).toEqual([11, 12])
  })
})
