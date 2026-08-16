import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const apiMocks = vi.hoisted(() => ({
  getById: vi.fn(),
  getSmartSchedule: vi.fn(),
  updateSmartSchedule: vi.fn(),
  copySmartSchedule: vi.fn(),
  listAccounts: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getQualityHardCloseSettings: vi.fn(),
  updateQualityHardCloseSettings: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getById: apiMocks.getById,
      getSmartSchedule: apiMocks.getSmartSchedule,
      updateSmartSchedule: apiMocks.updateSmartSchedule,
      copySmartSchedule: apiMocks.copySmartSchedule
    },
    accounts: {
      list: apiMocks.listAccounts,
      getBatchQualityStats: apiMocks.getBatchQualityStats,
      getBatchTodayStats: apiMocks.getBatchTodayStats,
      update: vi.fn(),
      setSchedulable: vi.fn(),
      resumeSmartSchedule: vi.fn()
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
    showWarning: vi.fn()
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
    template: `
      <div data-testid="smart-schedule-pool-table">
        <div data-testid="smart-schedule-pool-headers">
          <span
            v-for="col in columns"
            :key="col.key"
            :data-column="col.key"
            :data-sortable="col.sortable ? 'true' : 'false'"
          >{{ col.label }}</span>
        </div>
        <div v-for="row in data" :key="row.id">
          <slot name="cell-name" :row="row" :value="row.name" />
          <slot name="cell-quality_ttft" :row="row" />
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
    props: ['clickable'],
    template: '<button data-testid="account-quality-cell-button" @click="$emit(\'click\')" />'
  }
}))
vi.mock('@/components/account/AccountTodayStatsCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountGroupsCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountUsageCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountInlineNumberCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/AccountCapacityCell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/account/CapacityBadge.vue', () => ({ default: { template: '<div />' } }))
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
  return w
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  localStorage.removeItem('smart-schedule-pool-hidden-columns')
  localStorage.removeItem('smart-schedule-pool-column-layout')
  apiMocks.getById.mockResolvedValue({ id: 99, email: 'u@example.com', username: 'u', role: 'user' })
  apiMocks.getSmartSchedule.mockResolvedValue(makeView())
  apiMocks.updateSmartSchedule.mockResolvedValue(makeView())
  apiMocks.copySmartSchedule.mockResolvedValue(makeView())
  apiMocks.listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
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
})
