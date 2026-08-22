import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import {
  SCHEDULE_ERROR_WHITELIST_FAMILY_IDS,
  defaultScheduleErrorWhitelist
} from '@/api/admin/settings'
import UsageErrorInspectDialog from '../UsageErrorInspectDialog.vue'

const {
  list,
  getStats,
  listErrorLogs,
  getById,
  resolve,
  showError,
  showSuccess,
  getScheduleErrorWhitelist,
  updateScheduleErrorWhitelist
} = vi.hoisted(() => ({
  list: vi.fn(),
  getStats: vi.fn(),
  listErrorLogs: vi.fn(),
  getById: vi.fn(),
  resolve: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  getScheduleErrorWhitelist: vi.fn(),
  updateScheduleErrorWhitelist: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    usage: { list, getStats },
    users: { getById }
  }
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: { listErrorLogs }
}))

vi.mock('@/api/admin/settings', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/admin/settings')>()
  return {
    ...actual,
    getScheduleErrorWhitelist: (...args: unknown[]) => getScheduleErrorWhitelist(...args),
    updateScheduleErrorWhitelist: (...args: unknown[]) =>
      updateScheduleErrorWhitelist(...args)
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showWarning: vi.fn(),
    showSuccess
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

vi.mock('vue-router', () => ({
  useRouter: () => ({ resolve }),
  useRoute: () => ({ query: {} })
}))

const BaseDialogStub = {
  props: ['show', 'title', 'closeOnEscape', 'closeOnClickOutside'],
  template:
    '<div v-if="show" data-testid="usage-error-inspect-dialog" :data-close-escape="String(closeOnEscape)" :data-close-outside="String(closeOnClickOutside)"><slot /></div>'
}

describe('UsageErrorInspectDialog', () => {
  beforeEach(() => {
    list.mockReset()
    getStats.mockReset()
    listErrorLogs.mockReset()
    getById.mockReset()
    resolve.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    getScheduleErrorWhitelist.mockReset()
    updateScheduleErrorWhitelist.mockReset()
    list.mockResolvedValue({ items: [{ id: 1 }], total: 1 })
    getScheduleErrorWhitelist.mockResolvedValue({
      families: { ...defaultScheduleErrorWhitelist().families }
    })
    updateScheduleErrorWhitelist.mockResolvedValue({
      families: {
        ...defaultScheduleErrorWhitelist().families,
        group_no_account: true
      }
    })
    getStats.mockResolvedValue({
      total_requests: 12,
      total_actual_cost: 1.2345,
      total_account_cost: 0.89,
      total_cost: 2
    })
    listErrorLogs.mockResolvedValue({ items: [{ id: 9 }], total: 1 })
    resolve.mockReturnValue({ href: '/admin/usage?account_id=7' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  const mountDialog = (props: Record<string, unknown> = {}) =>
    mount(UsageErrorInspectDialog, {
      props: {
        show: true,
        scope: 'account',
        subjectId: 7,
        subjectLabel: 'acct-seven',
        initialTab: 'usage',
        ...props
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          DateRangePicker: true,
          Pagination: true,
          UsageFilters: {
            template: '<div data-test="usage-filters"><slot name="after-reset" /></div>'
          },
          UsageTable: {
            emits: ['userClick', 'userViewClick', 'sort'],
            template:
              '<div><button data-test="emit-user-click" type="button" @click="$emit(\'userClick\', 3)">user</button><button data-test="emit-user-view" type="button" @click="$emit(\'userViewClick\', 8)">view</button></div>'
          },
          ErrorRequestFilters: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: {
            props: ['show', 'errorId', 'errorType', 'zIndex'],
            template: '<div data-test="error-detail" :data-z="zIndex" :data-show="String(show)" />'
          },
          UserViewCompareDrawer: {
            props: ['logId', 'open', 'zIndex'],
            template: '<div data-test="user-view" :data-z="zIndex" :data-open="String(open)" />'
          },
          UserBalanceHistoryModal: {
            props: ['show', 'user', 'hideActions', 'zIndex'],
            template: '<div data-test="balance-history" :data-z="zIndex" :data-show="String(show)" />'
          }
        }
      }
    })

  it('loads usage logs for the locked account and does not fetch errors until that tab is opened', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(list).toHaveBeenCalledWith(
      expect.objectContaining({ account_id: 7 }),
      expect.anything()
    )
    expect(listErrorLogs).not.toHaveBeenCalled()

    const tabs = wrapper.findAll('[data-testid="inspect-detail-tab"]')
    expect(tabs).toHaveLength(3)
    expect(wrapper.get('[data-test="inspect-tab-whitelist"]').text()).toContain(
      'admin.ops.scheduleErrorWhitelist.title'
    )
    await tabs[1].trigger('click')
    await flushPromises()

    expect(listErrorLogs).toHaveBeenCalledWith(expect.objectContaining({
      account_id: 7,
      include_recovered: 'true'
    }))
    wrapper.unmount()
  })

  it('opens the full usage page in a new tab with the current subject and errors tab', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    resolve.mockReturnValue({ href: '/admin/usage?tab=errors&account_id=7' })
    const wrapper = mountDialog({ initialTab: 'errors' })
    await flushPromises()

    await wrapper.get('[data-testid="inspect-view-full"]').trigger('click')
    expect(resolve).toHaveBeenCalledWith({
      path: '/admin/usage',
      query: expect.objectContaining({
        account_id: '7',
        account_name: 'acct-seven',
        tab: 'errors'
      })
    })
    expect(openSpy).toHaveBeenCalledWith(
      '/admin/usage?tab=errors&account_id=7',
      '_blank',
      'noopener,noreferrer'
    )
    wrapper.unmount()
  })

  it('seeds a user-scoped dialog with user_id instead of account_id', async () => {
    const wrapper = mountDialog({
      scope: 'user',
      subjectId: 42,
      subjectLabel: 'user@example.com',
      initialTab: 'usage'
    })
    await flushPromises()
    expect(list).toHaveBeenCalledWith(
      expect.objectContaining({ user_id: 42 }),
      expect.anything()
    )
    expect(getStats).toHaveBeenCalledWith(expect.objectContaining({ user_id: 42 }))
    wrapper.unmount()
  })

  it('renders selected-range cost summary beside the date range', async () => {
    const wrapper = mountDialog({
      scope: 'user',
      subjectId: 42,
      subjectLabel: 'user@example.com'
    })
    await flushPromises()

    const summary = wrapper.get('[data-testid="inspect-usage-cost-summary"]')
    expect(summary.text()).toContain('$1.2345')
    expect(summary.text()).toContain('$0.8900')
    expect(summary.text()).toContain('12')
    expect(getStats).toHaveBeenCalledWith(expect.objectContaining({ user_id: 42 }))
    wrapper.unmount()
  })

  it('loads locked-account stats next to the shared date range', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    expect(getStats).toHaveBeenCalledWith(expect.objectContaining({ account_id: 7 }))
    wrapper.unmount()
  })

  it('raises nested detail overlays above the inspect dialog', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    expect(wrapper.get('[data-test="balance-history"]').attributes('data-z')).toBe('60')
    expect(wrapper.get('[data-test="user-view"]').attributes('data-z')).toBe('60')
    expect(wrapper.get('[data-test="error-detail"]').attributes('data-z')).toBe('60')
    wrapper.unmount()
  })

  it('dismisses nested overlays when the inspect dialog closes', async () => {
    getById.mockResolvedValue({ id: 3, email: 'u@example.com' })
    const wrapper = mountDialog()
    await flushPromises()

    await wrapper.get('[data-test="emit-user-click"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="balance-history"]').attributes('data-show')).toBe('true')

    await wrapper.get('[data-test="emit-user-view"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="user-view"]').attributes('data-open')).toBe('true')

    await wrapper.setProps({ show: false })
    await flushPromises()
    expect(wrapper.get('[data-test="balance-history"]').attributes('data-show')).toBe('false')
    expect(wrapper.get('[data-test="user-view"]').attributes('data-open')).toBe('false')
    wrapper.unmount()
  })

  it('disables inspect-dialog escape and outside-click while a nested overlay is open', async () => {
    getById.mockResolvedValue({ id: 3, email: 'u@example.com' })
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.get('[data-testid="usage-error-inspect-dialog"]').attributes('data-close-escape')).toBe('true')
    expect(wrapper.get('[data-testid="usage-error-inspect-dialog"]').attributes('data-close-outside')).toBe('true')

    await wrapper.get('[data-test="emit-user-click"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="usage-error-inspect-dialog"]').attributes('data-close-escape')).toBe('false')
    expect(wrapper.get('[data-testid="usage-error-inspect-dialog"]').attributes('data-close-outside')).toBe('false')
    wrapper.unmount()
  })

  it('aborts in-flight usage list requests on close and does not toast axios cancellation', async () => {
    list.mockImplementation((_params: unknown, options?: { signal?: AbortSignal }) => {
      return new Promise((_resolve, reject) => {
        options?.signal?.addEventListener('abort', () => {
          reject(Object.assign(new Error('canceled'), { name: 'CanceledError', code: 'ERR_CANCELED' }))
        })
      })
    })

    const wrapper = mountDialog()
    await flushPromises()
    expect(list).toHaveBeenCalled()

    await wrapper.setProps({ show: false })
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('loads seven new-family checkboxes on the schedule error whitelist tab', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.find('[data-test="schedule-error-whitelist-card"]').exists()).toBe(false)
    expect(getScheduleErrorWhitelist).not.toHaveBeenCalled()

    await wrapper.get('[data-test="inspect-tab-whitelist"]').trigger('click')
    await flushPromises()

    const card = wrapper.get('[data-test="schedule-error-whitelist-card"]')
    expect(card.text()).toContain('admin.ops.scheduleErrorWhitelist.checkedHint')
    expect(SCHEDULE_ERROR_WHITELIST_FAMILY_IDS).toHaveLength(7)
    expect(SCHEDULE_ERROR_WHITELIST_FAMILY_IDS).not.toContain('routing_model_miss')
    for (const id of SCHEDULE_ERROR_WHITELIST_FAMILY_IDS) {
      expect(wrapper.find(`[data-test="schedule-error-whitelist-${id}"]`).exists()).toBe(true)
    }
    expect(wrapper.find('[data-test="schedule-error-whitelist-routing_model_miss"]').exists()).toBe(
      false
    )
    expect(getScheduleErrorWhitelist).toHaveBeenCalledTimes(1)
    expect(listErrorLogs).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('uses factory defaults when whitelist config is missing', async () => {
    getScheduleErrorWhitelist.mockRejectedValueOnce(new Error('missing'))
    const wrapper = mountDialog()
    await flushPromises()

    await wrapper.get('[data-test="inspect-tab-whitelist"]').trigger('click')
    await flushPromises()

    for (const id of SCHEDULE_ERROR_WHITELIST_FAMILY_IDS) {
      const box = wrapper.get(`[data-test="schedule-error-whitelist-${id}"] input`)
      expect((box.element as HTMLInputElement).checked).toBe(false)
    }
    wrapper.unmount()
  })

  it('saves preset family checkboxes through the existing settings API', async () => {
    const wrapper = mountDialog({
      scope: 'user',
      subjectId: 42,
      subjectLabel: 'user@example.com',
      initialTab: 'errors'
    })
    await flushPromises()

    await wrapper.get('[data-test="inspect-tab-whitelist"]').trigger('click')
    await flushPromises()

    const groupBox = wrapper.get(
      '[data-test="schedule-error-whitelist-group_no_account"] input'
    )
    await groupBox.setValue(true)
    await wrapper.get('[data-test="schedule-error-whitelist-save"]').trigger('click')
    await flushPromises()

    expect(updateScheduleErrorWhitelist).toHaveBeenCalledTimes(1)
    expect(updateScheduleErrorWhitelist).toHaveBeenCalledWith({
      families: {
        ...defaultScheduleErrorWhitelist().families,
        group_no_account: true
      }
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.ops.scheduleErrorWhitelist.saved')
    wrapper.unmount()
  })
})
