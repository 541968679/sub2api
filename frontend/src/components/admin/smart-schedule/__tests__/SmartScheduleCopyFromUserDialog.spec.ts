import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import SmartScheduleCopyFromUserDialog from '../SmartScheduleCopyFromUserDialog.vue'

const apiMocks = vi.hoisted(() => ({
  list: vi.fn(),
  previewSmartScheduleCopyFromUser: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: apiMocks.list,
      previewSmartScheduleCopyFromUser: apiMocks.previewSmartScheduleCopyFromUser
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) =>
      params?.count != null ? `${key}:${params.count}` : key
  })
}))

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    props: ['show', 'title'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>'
  }
}))

function previewPayload(overrides: Record<string, unknown> = {}) {
  return {
    source_revision: 'rev-1',
    skipped_unavailable: 0,
    add: [12],
    remove: [13],
    overlap: [11],
    source_paused_account_ids: [11],
    enabled_delta: 'enable',
    source_empty: false,
    source_members: [],
    target_members: [],
    ...overrides
  }
}

describe('SmartScheduleCopyFromUserDialog', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    apiMocks.list.mockReset()
    apiMocks.previewSmartScheduleCopyFromUser.mockReset()
    apiMocks.list.mockResolvedValue({
      items: [
        { id: 16, email: 'src@example.com', username: 'src' },
        { id: 99, email: 'self@example.com', username: 'self' }
      ],
      total: 2
    })
    apiMocks.previewSmartScheduleCopyFromUser.mockResolvedValue(previewPayload())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('defaults pool concurrency and sort on, hides self, and confirms slices', async () => {
    const w = mount(SmartScheduleCopyFromUserDialog, {
      props: { show: true, targetUserId: 99, platform: 'anthropic' }
    })
    await w.get('[data-testid="smart-schedule-copy-from-user-search"]').setValue('src')
    await vi.advanceTimersByTimeAsync(280)
    await flushPromises()
    expect(apiMocks.list).toHaveBeenCalled()
    expect(w.find('[data-testid="smart-schedule-copy-from-user-option-99"]').exists()).toBe(false)
    await w.get('[data-testid="smart-schedule-copy-from-user-option-16"]').trigger('click')
    await flushPromises()
    expect(apiMocks.previewSmartScheduleCopyFromUser).toHaveBeenCalledWith(99, 'anthropic', 16)
    expect((w.get('[data-testid="smart-schedule-copy-from-user-slice-pool"]').element as HTMLInputElement).checked).toBe(true)
    expect((w.get('[data-testid="smart-schedule-copy-from-user-slice-concurrency"]').element as HTMLInputElement).checked).toBe(true)
    expect((w.get('[data-testid="smart-schedule-copy-from-user-slice-sort"]').element as HTMLInputElement).checked).toBe(true)
    expect((w.get('[data-testid="smart-schedule-copy-from-user-slice-thresholds"]').element as HTMLInputElement).checked).toBe(false)
    expect((w.get('[data-testid="smart-schedule-copy-from-user-slice-enabled"]').element as HTMLInputElement).checked).toBe(false)
    await w.get('[data-testid="smart-schedule-copy-from-user-confirm"]').trigger('click')
    expect(w.emitted('confirm')?.[0]?.[0]).toEqual({
      source_user_id: 16,
      source_revision: 'rev-1',
      slices: {
        pool: true,
        concurrency: true,
        sort_order: true,
        thresholds: false,
        enabled: false
      }
    })
  })

  it('disables concurrency and sort when pool is unchecked', async () => {
    const w = mount(SmartScheduleCopyFromUserDialog, {
      props: { show: true, targetUserId: 99, platform: 'anthropic' }
    })
    await w.get('[data-testid="smart-schedule-copy-from-user-search"]').setValue('src')
    await vi.advanceTimersByTimeAsync(280)
    await flushPromises()
    await w.get('[data-testid="smart-schedule-copy-from-user-option-16"]').trigger('click')
    await flushPromises()
    await w.get('[data-testid="smart-schedule-copy-from-user-slice-pool"]').setValue(false)
    await nextTick()
    const concurrency = w.get('[data-testid="smart-schedule-copy-from-user-slice-concurrency"]')
    const sort = w.get('[data-testid="smart-schedule-copy-from-user-slice-sort"]')
    expect((concurrency.element as HTMLInputElement).disabled).toBe(true)
    expect((sort.element as HTMLInputElement).disabled).toBe(true)
    expect((concurrency.element as HTMLInputElement).checked).toBe(false)
    expect((sort.element as HTMLInputElement).checked).toBe(false)
  })

  it('blocks confirm when the source pool is empty', async () => {
    apiMocks.previewSmartScheduleCopyFromUser.mockResolvedValue(previewPayload({ source_empty: true, add: [] }))
    const w = mount(SmartScheduleCopyFromUserDialog, {
      props: { show: true, targetUserId: 99, platform: 'anthropic' }
    })
    await w.get('[data-testid="smart-schedule-copy-from-user-search"]').setValue('src')
    await vi.advanceTimersByTimeAsync(280)
    await flushPromises()
    await w.get('[data-testid="smart-schedule-copy-from-user-option-16"]').trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="smart-schedule-copy-from-user-empty-source"]').exists()).toBe(true)
    expect((w.get('[data-testid="smart-schedule-copy-from-user-confirm"]').element as HTMLButtonElement).disabled).toBe(true)
  })
})
