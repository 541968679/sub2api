import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SmartSchedulePoolAddBar from '../SmartSchedulePoolAddBar.vue'
import type { Account } from '@/types'

vi.mock('@/components/common/HelpTooltip.vue', () => ({ default: { template: '<span />' } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (params) return key.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? ''))
        return key
      }
    })
  }
})

const intervals = [5, 10, 15, 30] as const

function account(overrides: Partial<Account> = {}): Account {
  return {
    id: 1,
    name: 'api-live',
    platform: 'anthropic',
    type: 'apikey',
    status: 'active',
    schedulable: true,
    proxy_id: null,
    concurrency: 1,
    priority: 0,
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '',
    updated_at: '',
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  } as Account
}

function mountBar(overrides: Record<string, unknown> = {}) {
  return mount(SmartSchedulePoolAddBar, {
    props: {
      searchQuery: '',
      searchOpen: false,
      filteredAccounts: [account()],
      apiCount: 1,
      oauthCount: 0,
      allCount: 1,
      poolEmpty: false,
      autoSorting: false,
      autoSortDone: 0,
      autoSortTotal: 0,
      refreshing: false,
      refreshDisabled: false,
      autoRefreshEnabled: false,
      autoRefreshCountdown: 0,
      autoRefreshIntervalSeconds: 5,
      autoRefreshIntervals: intervals,
      autoSortEnabled: false,
      ...overrides
    }
  })
}

describe('SmartSchedulePoolAddBar', () => {
  it('keeps add controls on the left and refresh ops on the right of a compact bar', () => {
    const w = mountBar()
    const region = w.get('[data-testid="smart-schedule-add-region"]')
    expect(region.classes()).toContain('w-fit')
    expect(region.classes()).toContain('max-w-full')
    expect(region.classes()).not.toContain('w-full')

    const left = region.get('[data-testid="smart-schedule-add-cluster"]')
    expect(left.get('[data-testid="smart-schedule-add-select"]').exists()).toBe(true)
    expect(left.get('[data-testid="smart-schedule-add-select"]').element.parentElement?.className).toContain('relative')
    expect(left.get('[data-testid="smart-schedule-filtered-add"]').exists()).toBe(true)
    expect(left.get('[data-testid="smart-schedule-add-api"]').exists()).toBe(true)
    expect(left.find('[data-testid="smart-schedule-refresh"]').exists()).toBe(false)
    expect(region.text()).toContain('admin.users.smartSchedule.addRegionTitle')
    expect(region.text()).toContain('admin.users.smartSchedule.addAccount')

    const ops = region.get('[data-testid="smart-schedule-add-ops"]')
    expect(ops.get('[data-testid="smart-schedule-refresh"]').exists()).toBe(true)
    expect(ops.get('[data-testid="smart-schedule-auto-refresh"]').exists()).toBe(true)
    expect(ops.get('[data-testid="smart-schedule-auto-sort"]').exists()).toBe(true)
    expect(ops.get('[data-testid="smart-schedule-interval-auto-sort"]').exists()).toBe(true)
    expect(ops.find('[data-testid="smart-schedule-filtered-add"]').exists()).toBe(false)
  })

  it('spins the auto-refresh trigger icon and shows the countdown while enabled', () => {
    const w = mountBar({ autoRefreshEnabled: true, autoRefreshCountdown: 4 })
    const icon = w.get('[data-testid="smart-schedule-auto-refresh-icon"]')
    expect(icon.classes()).toContain('animate-spin')
    expect(w.get('[data-testid="smart-schedule-auto-refresh"]').text()).toContain(
      'admin.accounts.autoRefreshCountdown'
    )

    const idle = mountBar({ autoRefreshEnabled: false })
    expect(idle.get('[data-testid="smart-schedule-auto-refresh-icon"]').classes()).not.toContain('animate-spin')
  })

  it('keeps one-click add clickable before candidates have loaded', async () => {
    const w = mountBar({ apiCount: 0, oauthCount: 0, allCount: 0, candidatesReady: false })
    expect(w.get('[data-testid="smart-schedule-add-api"]').attributes('disabled')).toBeUndefined()
    expect(w.get('[data-testid="smart-schedule-add-all"]').attributes('disabled')).toBeUndefined()
    expect(w.get('[data-testid="smart-schedule-filtered-add"]').attributes('disabled')).toBeUndefined()
    await w.get('[data-testid="smart-schedule-add-api"]').trigger('click')
    expect(w.emitted('add-scheduling')?.[0]).toEqual(['apikey'])
  })

  it('emits refresh and auto-refresh setting changes', async () => {
    const w = mountBar()
    await w.get('[data-testid="smart-schedule-refresh"]').trigger('click')
    expect(w.emitted('refresh')).toHaveLength(1)

    await w.get('[data-testid="smart-schedule-auto-refresh"]').trigger('click')
    expect(w.get('[data-testid="smart-schedule-auto-refresh-menu"]').exists()).toBe(true)
    await w.get('[data-testid="smart-schedule-auto-refresh-menu"]').findAll('button')[0].trigger('click')
    expect(w.emitted('set-auto-refresh-enabled')?.[0]).toEqual([true])
  })

  it('spins the interval auto-sort icon and shares the auto-refresh countdown', () => {
    const w = mountBar({
      autoSortEnabled: true,
      autoRefreshEnabled: true,
      autoRefreshCountdown: 3
    })
    expect(w.get('[data-testid="smart-schedule-interval-auto-sort-icon"]').classes()).toContain('animate-spin')
    expect(w.get('[data-testid="smart-schedule-interval-auto-sort"]').text()).toContain(
      'admin.users.smartSchedule.autoSortCountdown'
    )

    const sortOnly = mountBar({ autoSortEnabled: true, autoRefreshEnabled: false })
    expect(sortOnly.get('[data-testid="smart-schedule-interval-auto-sort-icon"]').classes()).toContain('animate-spin')
    expect(sortOnly.get('[data-testid="smart-schedule-interval-auto-sort"]').text()).toContain(
      'admin.users.smartSchedule.autoSort'
    )
    expect(sortOnly.get('[data-testid="smart-schedule-interval-auto-sort"]').text()).not.toContain(
      'admin.users.smartSchedule.autoSortCountdown'
    )

    const idle = mountBar({ autoSortEnabled: false })
    expect(idle.get('[data-testid="smart-schedule-interval-auto-sort-icon"]').classes()).not.toContain('animate-spin')
    expect(idle.get('[data-testid="smart-schedule-auto-sort"]').text()).toContain(
      'admin.users.smartSchedule.manualSort'
    )
  })

  it('toggles interval auto-sort without its own interval menu', async () => {
    const w = mountBar()
    await w.get('[data-testid="smart-schedule-interval-auto-sort"]').trigger('click')
    expect(w.find('[data-testid="smart-schedule-interval-auto-sort-menu"]').exists()).toBe(false)
    expect(w.emitted('set-auto-sort-enabled')?.[0]).toEqual([true])
  })
})
