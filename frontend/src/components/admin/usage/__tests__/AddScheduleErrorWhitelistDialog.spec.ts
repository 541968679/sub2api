import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { OpsErrorLog } from '@/api/admin/ops'
import AddScheduleErrorWhitelistDialog from '../AddScheduleErrorWhitelistDialog.vue'

const { addScheduleErrorWhitelistFromError, showSuccess, showError } = vi.hoisted(() => ({
  addScheduleErrorWhitelistFromError: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin/settings', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/admin/settings')>()
  return {
    ...actual,
    addScheduleErrorWhitelistFromError: (...args: unknown[]) =>
      addScheduleErrorWhitelistFromError(...args)
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="schedule-error-whitelist-from-log"><slot /><slot name="footer" /></div>'
}

const row: OpsErrorLog = {
  id: 9,
  created_at: '2026-08-23T00:00:00Z',
  phase: 'upstream',
  type: 'upstream_error',
  error_owner: 'provider',
  error_source: 'upstream_http',
  severity: 'error',
  status_code: 400,
  platform: 'openai',
  model: 'gpt-5.4',
  resolved: false,
  client_request_id: '',
  request_id: 'req',
  message: 'mapped',
  user_email: '',
  account_name: '',
  group_name: '',
  provider_error_code: 'channel:no_available_key',
  upstream_error_message: 'no available key'
}

describe('AddScheduleErrorWhitelistDialog', () => {
  beforeEach(() => {
    addScheduleErrorWhitelistFromError.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    addScheduleErrorWhitelistFromError.mockResolvedValue({ families: {}, custom: [] })
  })

  it('submits a structured rule from the log', async () => {
    const wrapper = mount(AddScheduleErrorWhitelistDialog, {
      props: { show: true, log: row },
      global: { stubs: { BaseDialog: BaseDialogStub } }
    })
    await wrapper.get('[data-test="schedule-error-whitelist-from-log-structured"]').trigger('click')
    await flushPromises()
    expect(addScheduleErrorWhitelistFromError).toHaveBeenCalledWith(9, 'structured')
    expect(showSuccess).toHaveBeenCalled()
  })

  it('blocks 502 upstream request failed', async () => {
    const wrapper = mount(AddScheduleErrorWhitelistDialog, {
      props: {
        show: true,
        log: {
          ...row,
          status_code: 502,
          message: 'Upstream request failed'
        }
      },
      global: { stubs: { BaseDialog: BaseDialogStub } }
    })
    expect(wrapper.find('[data-test="schedule-error-whitelist-from-log-blocked"]').exists()).toBe(true)
    expect(
      (wrapper.get('[data-test="schedule-error-whitelist-from-log-structured"]').element as HTMLButtonElement).disabled
    ).toBe(true)
  })
})
