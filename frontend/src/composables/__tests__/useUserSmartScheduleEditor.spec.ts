import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { isCurrentlySchedulingAccount, useUserSmartScheduleEditor } from '../useUserSmartScheduleEditor'
import { resolvePairCap } from '../smartSchedulePoolAdmission'

const apiMocks = vi.hoisted(() => ({
  getSmartSchedule: vi.fn(),
  listAccounts: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getBatchTodayStats: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { getSmartSchedule: apiMocks.getSmartSchedule },
    accounts: {
      list: apiMocks.listAccounts,
      getBatchQualityStats: apiMocks.getBatchQualityStats,
      getBatchTodayStats: apiMocks.getBatchTodayStats
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

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/composables/useQualityThresholdTemplate', () => ({
  useQualityThresholdTemplate: () => ({
    templateBusy: { value: false },
    applyQualityTemplate: vi.fn(),
    saveQualityTemplate: vi.fn()
  })
}))

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

function mountEditor(userId = 99) {
  const Comp = defineComponent({
    setup() {
      const id = ref<number | null>(userId)
      return useUserSmartScheduleEditor(id)
    },
    template: '<div />'
  })
  return mount(Comp)
}

describe('isCurrentlySchedulingAccount', () => {
  it('accepts active schedulable accounts', () => {
    expect(isCurrentlySchedulingAccount({ status: 'active', schedulable: true })).toBe(true)
  })

  it('rejects paused, unschedulable, or temporarily blocked accounts', () => {
    expect(isCurrentlySchedulingAccount({ status: 'inactive', schedulable: true })).toBe(false)
    expect(isCurrentlySchedulingAccount({ status: 'active', schedulable: false })).toBe(false)
    expect(
      isCurrentlySchedulingAccount({
        status: 'active',
        schedulable: true,
        temp_unschedulable_until: new Date(Date.now() + 60_000).toISOString()
      })
    ).toBe(false)
    expect(
      isCurrentlySchedulingAccount({
        status: 'active',
        schedulable: true,
        rate_limit_reset_at: new Date(Date.now() + 60_000).toISOString()
      })
    ).toBe(false)
  })
})

describe('effective pair cap display', () => {
  it('does not fall back to account-wide concurrency', () => {
    expect(resolvePairCap(null)).toBeNull()
    expect(resolvePairCap(0)).toBeNull()
    expect(resolvePairCap(4)).toBe(4)
  })
})

describe('useUserSmartScheduleEditor loadAll', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 21, platform: 'openai', max_concurrency: 2 }]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 21, name: 'oa-1', platform: 'openai', type: 'oauth', status: 'active' }],
      total: 1,
      page: 1,
      page_size: 1,
      pages: 1
    })
    apiMocks.getBatchQualityStats.mockResolvedValue({ stats: {} })
    apiMocks.getBatchTodayStats.mockResolvedValue({ stats: {} })
  })

  it('does not wait for candidates and skips the default-platform watch reload', async () => {
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.initialLoaded).toBe(true)
    expect(w.vm.loading).toBe(false)
    expect(w.vm.activePlatform).toBe('openai')
    const listCalls = apiMocks.listAccounts.mock.calls as Array<
      [number, number, { ids?: string; lite?: string; platform?: string }]
    >
    expect(listCalls.filter((call) => !call[2]?.ids)).toHaveLength(0)
    expect(listCalls.filter((call) => call[2]?.ids === '21')).toHaveLength(1)
    expect(listCalls[0]?.[2]).toMatchObject({ lite: '1', platform: 'openai' })
  })

  it('loads lite candidates only when the add UI asks', async () => {
    const w = mountEditor()
    await flushPromises()
    apiMocks.listAccounts.mockClear()
    apiMocks.listAccounts.mockResolvedValue({
      items: [{ id: 31, name: 'cand', platform: 'openai', type: 'apikey', status: 'active', schedulable: true }],
      total: 1,
      page: 1,
      page_size: 1000,
      pages: 1
    })
    await w.vm.ensureCandidates()
    await flushPromises()
    expect(apiMocks.listAccounts).toHaveBeenCalledWith(
      1,
      1000,
      expect.objectContaining({ platform: 'openai', lite: '1' })
    )
    expect(w.vm.candidatesReady).toBe(true)
  })

  it('silent refresh does not flip the first-paint loading flag', async () => {
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.loading).toBe(false)
    const pending = w.vm.refreshAll({ silent: true })
    expect(w.vm.loading).toBe(false)
    expect(w.vm.refreshing).toBe(true)
    await pending
    expect(w.vm.loading).toBe(false)
    expect(w.vm.refreshing).toBe(false)
  })
})
