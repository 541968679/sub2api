import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { isCurrentlySchedulingAccount, useUserSmartScheduleEditor } from '../useUserSmartScheduleEditor'
import { resolvePairCap } from '../smartSchedulePoolAdmission'

const apiMocks = vi.hoisted(() => ({
  getSmartSchedule: vi.fn(),
  updateSmartSchedule: vi.fn(),
  updateSmartScheduleSortOrder: vi.fn(),
  listAccounts: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getSmartSchedulePnlPairs: vi.fn(),
  getSmartSchedulePairQualityBatch: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getSmartSchedule: apiMocks.getSmartSchedule,
      updateSmartSchedule: apiMocks.updateSmartSchedule,
      updateSmartScheduleSortOrder: apiMocks.updateSmartScheduleSortOrder,
      getSmartSchedulePnlPairs: apiMocks.getSmartSchedulePnlPairs,
      getSmartSchedulePairQualityBatch: apiMocks.getSmartSchedulePairQualityBatch
    },
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
    quality_window_n: null,
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
    apiMocks.getSmartSchedulePnlPairs.mockResolvedValue({ pairs: {} })
    apiMocks.getSmartSchedulePairQualityBatch.mockResolvedValue({ pairs: {} })
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

  it('keeps membership sort_order from GET and persists it without touching account priority', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 21, platform: 'openai', max_concurrency: 2, sort_order: 2 },
            { account_id: 22, platform: 'openai', max_concurrency: null, sort_order: 1 }
          ]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
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
    apiMocks.updateSmartScheduleSortOrder.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 22, platform: 'openai', max_concurrency: null, sort_order: 1 },
            { account_id: 21, platform: 'openai', max_concurrency: 2, sort_order: 2 }
          ]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.memberSortOrder(22)).toBe(1)
    expect(w.vm.memberSortOrder(21)).toBe(2)
    const ok = await w.vm.persistSortOrders([
      { account_id: 22, sort_order: 1 },
      { account_id: 21, sort_order: 2 }
    ])
    expect(ok).toBe(true)
    expect(apiMocks.updateSmartScheduleSortOrder).toHaveBeenCalledWith(99, 'openai', {
      accounts: [
        { account_id: 22, sort_order: 1 },
        { account_id: 21, sort_order: 2 }
      ]
    })
    expect(w.vm.memberSortOrder(22)).toBe(1)
    expect(w.vm.memberSortOrder(21)).toBe(2)
  })

  it('persists pool membership immediately when adding an account', async () => {
    apiMocks.updateSmartSchedule.mockImplementation(
      (_userId: number, _platform: string, body: { accounts?: Array<{ account_id: number; max_concurrency?: number | null }> }) =>
        Promise.resolve({
          user_id: 99,
          default_platform: 'openai',
          platforms: {
            anthropic: emptyPlatform(),
            openai: {
              ...emptyPlatform(),
              enabled: true,
              accounts: body.accounts ?? []
            },
            gemini: emptyPlatform(),
            antigravity: emptyPlatform(),
            grok: emptyPlatform()
          }
        })
    )
    const w = mountEditor()
    await flushPromises()
    await w.vm.addAccountById(31)
    await flushPromises()
    expect(apiMocks.updateSmartSchedule).toHaveBeenCalledWith(
      99,
      'openai',
      expect.objectContaining({
        accounts: expect.arrayContaining([
          expect.objectContaining({ account_id: 21 }),
          expect.objectContaining({ account_id: 31 })
        ])
      })
    )
    expect(w.vm.isDirty).toBe(false)
  })

  it('keeps pair-cap edits dirty until save', async () => {
    const w = mountEditor()
    await flushPromises()
    w.vm.setMemberCap(21, 4)
    expect(w.vm.isDirty).toBe(true)
    expect(apiMocks.updateSmartSchedule).not.toHaveBeenCalled()
  })

  it('still loads pool accounts when schedule pnl request fails', async () => {
    apiMocks.getSmartSchedulePnlPairs.mockRejectedValueOnce(new Error('pnl down'))
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.poolAccounts).toEqual([
      expect.objectContaining({ id: 21, name: 'oa-1' })
    ])
    expect(w.vm.pairPnlById).toEqual({})
  })

  it('loads pair quality independently of account 15m quality', async () => {
    apiMocks.getSmartSchedulePairQualityBatch.mockResolvedValue({
      pairs: {
        '21': { ttft_p50_ms: 180, success_rate: 0.95, ttft_samples: 4, ok_samples: 6, n: 10 }
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(apiMocks.getSmartSchedulePairQualityBatch).toHaveBeenCalledWith(99, [21])
    expect(w.vm.pairQualityById['21']).toEqual({
      ttft_p50_ms: 180,
      success_rate: 0.95,
      ttft_samples: 4,
      ok_samples: 6,
      n: 10
    })
  })

  it('maps a single window N from GET, preferring the new field', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          quality_window_n: 14,
          quality_min_success_samples: 3,
          quality_min_ttft_samples: 4,
          accounts: [{ account_id: 21, platform: 'openai', max_concurrency: 2 }]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.currentDraft.windowN).toBe(14)
  })
})
