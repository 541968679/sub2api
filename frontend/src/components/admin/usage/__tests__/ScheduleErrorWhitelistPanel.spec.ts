import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import {
  defaultScheduleErrorWhitelist,
  SCHEDULE_ERROR_WHITELIST_FAMILY_IDS
} from '@/api/admin/settings'
import ScheduleErrorWhitelistPanel from '../ScheduleErrorWhitelistPanel.vue'

const { getScheduleErrorWhitelist, updateScheduleErrorWhitelist, showSuccess } = vi.hoisted(() => ({
  getScheduleErrorWhitelist: vi.fn(),
  updateScheduleErrorWhitelist: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin/settings', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/admin/settings')>()
  return {
    ...actual,
    getScheduleErrorWhitelist: (...args: unknown[]) => getScheduleErrorWhitelist(...args),
    updateScheduleErrorWhitelist: (...args: unknown[]) => updateScheduleErrorWhitelist(...args)
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('ScheduleErrorWhitelistPanel', () => {
  beforeEach(() => {
    getScheduleErrorWhitelist.mockReset()
    updateScheduleErrorWhitelist.mockReset()
    showSuccess.mockReset()
    getScheduleErrorWhitelist.mockResolvedValue({
      families: { ...defaultScheduleErrorWhitelist().families },
      custom: []
    })
    updateScheduleErrorWhitelist.mockImplementation(async (payload: unknown) => payload)
  })

  it('saves preset families without sending custom', async () => {
    const wrapper = mount(ScheduleErrorWhitelistPanel)
    await flushPromises()

    await wrapper.get('[data-test="schedule-error-whitelist-group_no_account"] input').setValue(true)
    await wrapper.get('[data-test="schedule-error-whitelist-save"]').trigger('click')
    await flushPromises()

    expect(updateScheduleErrorWhitelist).toHaveBeenCalledWith({
      families: {
        ...defaultScheduleErrorWhitelist().families,
        group_no_account: true
      }
    })
    expect(SCHEDULE_ERROR_WHITELIST_FAMILY_IDS).toHaveLength(7)
  })

  it('adds a message-contains custom rule', async () => {
    const wrapper = mount(ScheduleErrorWhitelistPanel)
    await flushPromises()

    await wrapper.get('[data-test="schedule-error-whitelist-custom-add-input"]').setValue('quota exceeded')
    await wrapper.get('[data-test="schedule-error-whitelist-custom-add"]').trigger('click')
    await flushPromises()

    expect(updateScheduleErrorWhitelist).toHaveBeenCalledWith({
      families: { ...defaultScheduleErrorWhitelist().families },
      custom: [{ enabled: true, message_contains: 'quota exceeded' }]
    })
  })
})
