import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import {
  isCurrentlySchedulingAccount,
  smartSchedulePoolAccountListFilters,
  useUserSmartScheduleEditor
} from '../useUserSmartScheduleEditor'
import { resolvePairCap } from '../smartSchedulePoolAdmission'

const apiMocks = vi.hoisted(() => ({
  getSmartSchedule: vi.fn(),
  updateSmartSchedule: vi.fn(),
  updateSmartScheduleSortOrder: vi.fn(),
  listAccounts: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getSmartSchedulePnlPairs: vi.fn(),
  getSmartSchedulePairQualityBatch: vi.fn(),
  getUsage: vi.fn(),
  resumeSmartSchedule: vi.fn()
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
      getBatchTodayStats: apiMocks.getBatchTodayStats,
      getUsage: apiMocks.getUsage,
      resumeSmartSchedule: apiMocks.resumeSmartSchedule
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
    probe_concurrency_mode: 'follow_n' as const,
    probe_concurrency: null,
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

describe('smartSchedulePoolAccountListFilters', () => {
  it('omits platform on the antigravity tab so mixed ids are not dropped', () => {
    expect(smartSchedulePoolAccountListFilters('antigravity', [51, 1730])).toEqual({
      ids: '51,1730',
      lite: '1'
    })
    expect(smartSchedulePoolAccountListFilters('antigravity', [1730]).platform).toBeUndefined()
  })

  it('keeps the tab platform on other pools', () => {
    expect(smartSchedulePoolAccountListFilters('openai', [21])).toEqual({
      ids: '21',
      lite: '1',
      platform: 'openai'
    })
    expect(smartSchedulePoolAccountListFilters('anthropic', [11])).toMatchObject({ platform: 'anthropic' })
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
    apiMocks.getUsage.mockResolvedValue({
      balance_usd: 12.5,
      balance_updated_at: '2026-08-22T05:00:00.000Z'
    })
    apiMocks.resumeSmartSchedule.mockImplementation(
      (accountId: number, userId: number, state = 'resumed') =>
        Promise.resolve({
          account_id: accountId,
          user_id: userId,
          state,
          probing: state === 'probing',
          pinned: state === 'pinned',
          probe_cap: state === 'probing' ? 2 : undefined,
          cooldown_until: null
        })
    )
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

  it('loads OpenAI plus native AG candidates on the antigravity tab', async () => {
    const w = mountEditor()
    await flushPromises()
    w.vm.activePlatform = 'antigravity'
    apiMocks.listAccounts.mockClear()
    apiMocks.listAccounts.mockImplementation(
      (_page: number, _size: number, filters?: { platform?: string }) => {
        if (filters?.platform === 'openai') {
          return Promise.resolve({
            items: [{ id: 41, name: 'oai', platform: 'openai', type: 'apikey', status: 'active' }],
            total: 1,
            page: 1,
            page_size: 1000,
            pages: 1
          })
        }
        return Promise.resolve({
          items: [{ id: 51, name: 'ag', platform: 'antigravity', type: 'oauth', status: 'active' }],
          total: 1,
          page: 1,
          page_size: 1000,
          pages: 1
        })
      }
    )
    await w.vm.ensureCandidates()
    await flushPromises()
    const platforms = (apiMocks.listAccounts.mock.calls as Array<[number, number, { platform?: string }]>)
      .map((call) => call[2]?.platform)
      .sort()
    expect(platforms).toEqual(['antigravity', 'openai'])
    expect(w.vm.addableAccounts.map((item: { id: number }) => item.id).sort()).toEqual([41, 51])
  })

  it('hydrates OpenAI members on the AG tab without platform=antigravity', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'antigravity',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 1730, platform: 'openai', max_concurrency: null }]
        },
        gemini: emptyPlatform(),
        antigravity: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 51, platform: 'antigravity', max_concurrency: null },
            { account_id: 1730, platform: 'antigravity', max_concurrency: null }
          ]
        },
        grok: emptyPlatform()
      }
    })
    apiMocks.listAccounts.mockImplementation(
      (_page: number, _size: number, filters?: { ids?: string; platform?: string }) => {
        const catalog = [
          {
            id: 51,
            name: 'ag-native',
            platform: 'antigravity',
            type: 'oauth',
            status: 'active'
          },
          {
            id: 1730,
            name: 'loveapi',
            platform: 'openai',
            type: 'apikey',
            status: 'active',
            extra: { openai_claude_gpt_bridge_enabled: true }
          }
        ]
        let items = catalog
        if (filters?.ids) {
          const wanted = new Set(filters.ids.split(',').map((id) => Number(id)))
          items = items.filter((item) => wanted.has(item.id))
        }
        if (filters?.platform) {
          items = items.filter((item) => item.platform === filters.platform)
        }
        return Promise.resolve({
          items,
          total: items.length,
          page: 1,
          page_size: items.length || 1,
          pages: 1
        })
      }
    )
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.activePlatform).toBe('antigravity')
    const poolCalls = (apiMocks.listAccounts.mock.calls as Array<
      [number, number, { ids?: string; lite?: string; platform?: string }]
    >).filter((call) => Boolean(call[2]?.ids))
    expect(poolCalls).toHaveLength(1)
    expect(poolCalls[0]?.[2]).toEqual({ ids: '51,1730', lite: '1' })
    expect(poolCalls[0]?.[2]?.platform).toBeUndefined()
    expect(w.vm.poolAccounts.map((item: { id: number; name: string }) => ({ id: item.id, name: item.name }))).toEqual([
      { id: 51, name: 'ag-native' },
      { id: 1730, name: 'loveapi' }
    ])
    expect(w.vm.currentDraft.accounts.map((item: { account_id: number }) => item.account_id)).toEqual([51, 1730])
  })

  it('keeps an OpenAI id in the AG draft after add and save, then rehydrates it', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'antigravity',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 1730, platform: 'openai', max_concurrency: null }]
        },
        gemini: emptyPlatform(),
        antigravity: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{ account_id: 51, platform: 'antigravity', max_concurrency: null }]
        },
        grok: emptyPlatform()
      }
    })
    const catalog = [
      {
        id: 51,
        name: 'ag-native',
        platform: 'antigravity',
        type: 'oauth',
        status: 'active',
        schedulable: true
      },
      {
        id: 1730,
        name: 'loveapi',
        platform: 'openai',
        type: 'apikey',
        status: 'active',
        schedulable: true,
        extra: { openai_claude_gpt_bridge_enabled: true }
      }
    ]
    apiMocks.listAccounts.mockImplementation(
      (_page: number, _size: number, filters?: { ids?: string; platform?: string }) => {
        let items = catalog
        if (filters?.ids) {
          const wanted = new Set(filters.ids.split(',').map((id) => Number(id)))
          items = items.filter((item) => wanted.has(item.id))
        }
        if (filters?.platform) {
          items = items.filter((item) => item.platform === filters.platform)
        }
        return Promise.resolve({
          items,
          total: items.length,
          page: 1,
          page_size: items.length || 1,
          pages: 1
        })
      }
    )
    apiMocks.updateSmartSchedule.mockImplementation(
      (_userId: number, platform: string, body: { accounts?: Array<{ account_id: number; max_concurrency?: number | null }> }) =>
        Promise.resolve({
          user_id: 99,
          default_platform: 'antigravity',
          platforms: {
            anthropic: emptyPlatform(),
            openai: {
              ...emptyPlatform(),
              enabled: true,
              accounts: [{ account_id: 1730, platform: 'openai', max_concurrency: null }]
            },
            gemini: emptyPlatform(),
            antigravity: {
              ...emptyPlatform(),
              enabled: true,
              accounts: (body.accounts ?? []).map((item) => ({
                ...item,
                platform
              }))
            },
            grok: emptyPlatform()
          }
        })
    )
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.poolAccounts.map((item: { id: number }) => item.id)).toEqual([51])
    await w.vm.addAccountById(1730)
    await flushPromises()
    expect(apiMocks.updateSmartSchedule).toHaveBeenCalledWith(
      99,
      'antigravity',
      expect.objectContaining({
        accounts: expect.arrayContaining([
          expect.objectContaining({ account_id: 51 }),
          expect.objectContaining({ account_id: 1730 })
        ])
      })
    )
    expect(w.vm.currentDraft.accounts.map((item: { account_id: number }) => item.account_id)).toEqual([51, 1730])
    const poolCalls = (apiMocks.listAccounts.mock.calls as Array<
      [number, number, { ids?: string; lite?: string; platform?: string }]
    >).filter((call) => Boolean(call[2]?.ids))
    expect(poolCalls.length).toBeGreaterThanOrEqual(2)
    for (const call of poolCalls) {
      expect(call[2]?.platform).not.toBe('antigravity')
      expect(call[2]).toMatchObject({ lite: '1' })
    }
    expect(poolCalls.at(-1)?.[2]).toEqual({ ids: '51,1730', lite: '1' })
    expect(w.vm.poolAccounts.map((item: { id: number; name: string }) => ({ id: item.id, name: item.name }))).toEqual([
      { id: 51, name: 'ag-native' },
      { id: 1730, name: 'loveapi' }
    ])
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

  it('probes stale api-key balances after load and can force a fresh snapshot', async () => {
    const now = Date.now()
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [
            { account_id: 21, platform: 'openai', max_concurrency: 2 },
            { account_id: 22, platform: 'openai', max_concurrency: 2 },
            { account_id: 23, platform: 'openai', max_concurrency: 2 }
          ]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    apiMocks.listAccounts.mockResolvedValue({
      items: [
        {
          id: 21,
          name: 'stale-key',
          platform: 'openai',
          type: 'apikey',
          extra: { upstream_balance_usd: 10, upstream_balance_at: new Date(now - 10 * 60 * 1000).toISOString() }
        },
        {
          id: 22,
          name: 'fresh-key',
          platform: 'openai',
          type: 'apikey',
          extra: { upstream_balance_usd: 20, upstream_balance_at: new Date(now - 2 * 60 * 1000).toISOString() }
        },
        { id: 23, name: 'oauth', platform: 'openai', type: 'oauth', extra: {} }
      ],
      total: 3,
      page: 1,
      page_size: 3,
      pages: 1
    })
    const w = mountEditor()
    await flushPromises()
    expect(apiMocks.getUsage).toHaveBeenCalledTimes(1)
    expect(apiMocks.getUsage).toHaveBeenCalledWith(21, 'active', undefined)
    expect(w.vm.poolAccounts.find((item: { id: number }) => item.id === 21)?.extra).toMatchObject({
      upstream_balance_usd: 12.5,
      upstream_balance_at: '2026-08-22T05:00:00.000Z'
    })
    apiMocks.getUsage.mockClear()
    await w.vm.refreshAccountBalance(22)
    await flushPromises()
    expect(apiMocks.getUsage).toHaveBeenCalledWith(22, 'active', { force: true })
  })

  it('loads pair quality independently of account 15m quality', async () => {
    apiMocks.getSmartSchedulePairQualityBatch.mockResolvedValue({
      pairs: {
        '21': { ttft_p50_ms: 180, success_rate: 0.95, ttft_samples: 4, ok_samples: 6, n: 10 }
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(apiMocks.getSmartSchedulePairQualityBatch).toHaveBeenCalledWith(99, [21], 'openai')
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
    expect(w.vm.currentDraft.probeConcurrencyMode).toBe('follow_n')
  })

  it('hydrates custom probe concurrency from GET and does not copy template N into it', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          quality_window_n: 14,
          probe_concurrency_mode: 'custom',
          probe_concurrency: 2,
          accounts: [{ account_id: 21, platform: 'openai', max_concurrency: 5 }]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.currentDraft.probeConcurrencyMode).toBe('custom')
    expect(w.vm.currentDraft.probeConcurrency).toBe(2)
    w.vm.applyTemplateToDraft({
      quality_max_p50_ttft_ms: 200,
      quality_min_success_rate_percent: 90,
      quality_min_success_samples: 20,
      quality_min_ttft_samples: 20,
      quality_condition: 'or'
    })
    expect(w.vm.currentDraft.probeConcurrencyMode).toBe('custom')
    expect(w.vm.currentDraft.probeConcurrency).toBe(2)
    expect(w.vm.currentDraft.windowN).toBe(14)
    expect(w.vm.currentDraft.maxP50).toBe(200)
  })

  it('writes follow_n by default and custom probe concurrency on save', async () => {
    apiMocks.updateSmartSchedule.mockImplementation(
      (_userId: number, _platform: string, body: Record<string, unknown>) =>
        Promise.resolve({
          user_id: 99,
          default_platform: 'openai',
          platforms: {
            anthropic: emptyPlatform(),
            openai: {
              ...emptyPlatform(),
              ...body,
              enabled: true,
              accounts: [{ account_id: 21, platform: 'openai', max_concurrency: 2 }]
            },
            gemini: emptyPlatform(),
            antigravity: emptyPlatform(),
            grok: emptyPlatform()
          }
        })
    )
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.currentDraft.probeConcurrencyMode).toBe('follow_n')
    await w.vm.onSave()
    expect(apiMocks.updateSmartSchedule).toHaveBeenCalledWith(
      99,
      'openai',
      expect.objectContaining({
        probe_concurrency_mode: 'follow_n',
        probe_concurrency: null
      })
    )
    w.vm.currentDraft.probeConcurrencyMode = 'custom'
    w.vm.currentDraft.probeConcurrency = 7
    await w.vm.onSave()
    expect(apiMocks.updateSmartSchedule).toHaveBeenLastCalledWith(
      99,
      'openai',
      expect.objectContaining({
        probe_concurrency_mode: 'custom',
        probe_concurrency: 7
      })
    )
  })

  it('rejects invalid custom probe concurrency instead of clamping', async () => {
    const w = mountEditor()
    await flushPromises()
    w.vm.currentDraft.probeConcurrencyMode = 'custom'
    w.vm.currentDraft.probeConcurrency = 0
    await w.vm.onSave()
    expect(apiMocks.updateSmartSchedule).not.toHaveBeenCalled()
    w.vm.currentDraft.probeConcurrency = 101
    await w.vm.onSave()
    expect(apiMocks.updateSmartSchedule).not.toHaveBeenCalled()
    w.vm.currentDraft.probeConcurrency = ''
    await w.vm.onSave()
    expect(apiMocks.updateSmartSchedule).not.toHaveBeenCalled()
  })

  it('hydrates probing from GET and only enters it when the next state is explicit', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          quality_window_n: 10,
          accounts: [{
            account_id: 21,
            platform: 'openai',
            max_concurrency: 8,
            paused: true,
            probing: false
          }]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.memberPaused(21)).toBe(true)
    expect(w.vm.memberProbing(21)).toBe(false)
    expect(w.vm.memberProbeCap(21)).toBeNull()

    await w.vm.setPairAdmission(21, 'selectable')
    await flushPromises()
    expect(apiMocks.resumeSmartSchedule).toHaveBeenCalledWith(21, 99, 'selectable', 'openai')
    expect(w.vm.memberPaused(21)).toBe(false)
    expect(w.vm.memberProbing(21)).toBe(false)

    await w.vm.setPairAdmission(21, 'probing')
    await flushPromises()
    expect(apiMocks.resumeSmartSchedule).toHaveBeenCalledWith(21, 99, 'probing', 'openai')
    expect(w.vm.memberProbing(21)).toBe(true)
    expect(w.vm.memberProbeCap(21)).toBe(2)
    expect(w.vm.memberResumeActive(21)).toBe(false)

    await w.vm.setPairAdmission(21, 'pinned')
    await flushPromises()
    expect(apiMocks.resumeSmartSchedule).toHaveBeenCalledWith(21, 99, 'pinned', 'openai')
    expect(w.vm.memberPinned(21)).toBe(true)
    expect(w.vm.memberProbing(21)).toBe(false)
    expect(w.vm.memberResumeActive(21)).toBe(false)
  })

  it('hydrates probing and probe_cap from GET without inventing them', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          quality_window_n: 10,
          accounts: [{
            account_id: 21,
            platform: 'openai',
            max_concurrency: 8,
            probing: true,
            probe_cap: 3
          }]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.memberProbing(21)).toBe(true)
    expect(w.vm.memberProbeCap(21)).toBe(3)
  })

  it('hydrates pinned from GET without inventing it from resume', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{
            account_id: 21,
            platform: 'openai',
            pinned: true
          }]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.memberPinned(21)).toBe(true)
    expect(w.vm.memberProbing(21)).toBe(false)
  })

  it('does not invent pin from an expired resume mark', async () => {
    apiMocks.getSmartSchedule.mockResolvedValue({
      user_id: 99,
      default_platform: 'openai',
      platforms: {
        anthropic: emptyPlatform(),
        openai: {
          ...emptyPlatform(),
          enabled: true,
          accounts: [{
            account_id: 21,
            platform: 'openai',
            admission: 'resumed',
            pinned: false
          }]
        },
        gemini: emptyPlatform(),
        antigravity: emptyPlatform(),
        grok: emptyPlatform()
      }
    })
    const w = mountEditor()
    await flushPromises()
    expect(w.vm.memberPinned(21)).toBe(false)
  })
})
