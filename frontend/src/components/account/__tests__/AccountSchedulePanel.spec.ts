import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const {
  listSmartScheduleMemberships,
  addSmartScheduleMember,
  setPublicSchedulable,
  setSchedulable,
  setSmartScheduleAdmissionBatch,
  showSuccess
} = vi.hoisted(() => ({
  listSmartScheduleMemberships: vi.fn(),
  addSmartScheduleMember: vi.fn(),
  setPublicSchedulable: vi.fn(),
  setSchedulable: vi.fn(),
  setSmartScheduleAdmissionBatch: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      listSmartScheduleMemberships,
      addSmartScheduleMember,
      removeSmartScheduleMember: vi.fn(),
      setSmartScheduleAdmissionBatch,
      setPublicSchedulable,
      setSchedulable,
      resumeSmartSchedule: vi.fn()
    }
  }
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

import AccountSchedulePanel from '../AccountSchedulePanel.vue'

const account = {
  id: 7,
  name: 'Claude',
  platform: 'anthropic',
  type: 'apikey',
  status: 'active',
  schedulable: true,
  public_schedulable: true
} as any

function mountPanel(accountOverride: Record<string, unknown> = {}) {
  return mount(AccountSchedulePanel, {
    props: { account: { ...account, ...accountOverride } },
    global: {
      stubs: {
        OpenAIFastPolicyUserSelector: defineComponent({
          name: 'OpenAIFastPolicyUserSelector',
          emits: ['select', 'update:modelValue'],
          template: '<button type="button" data-testid="add-user" @click="$emit(\'select\', { id: 16, email: \'a@x.com\', deleted: false })" />'
        }),
        SmartScheduleAdmissionSwitch: defineComponent({
          name: 'SmartScheduleAdmissionSwitch',
          props: ['disabled'],
          emits: ['select'],
          template:
            '<button type="button" :disabled="disabled" data-testid="admission-switch" @click="$emit(\'select\', \'paused\')" />'
        })
      }
    }
  })
}

describe('AccountSchedulePanel', () => {
  it('loads members for the current platform and can add one', async () => {
    listSmartScheduleMemberships.mockResolvedValue([
      { user_id: 16, email: 'a@x.com', deleted: false, platform: 'anthropic', enabled: false, paused: false }
    ])
    addSmartScheduleMember.mockResolvedValue(undefined)
    const wrapper = mountPanel()
    await flushPromises()
    expect(listSmartScheduleMemberships).toHaveBeenCalledWith(7, 'anthropic')
    expect(wrapper.get('[data-testid="account-schedule-member-16"]').text()).toContain('a@x.com')
    await wrapper.get('[data-testid="add-user"]').trigger('click')
    await flushPromises()
    expect(addSmartScheduleMember).toHaveBeenCalledWith(7, 16, 'anthropic')
  })

  it('persists the public-pool switch immediately', async () => {
    listSmartScheduleMemberships.mockResolvedValue([])
    setPublicSchedulable.mockResolvedValue({ ...account, public_schedulable: false })
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-testid="account-schedule-public"]').trigger('click')
    await flushPromises()
    expect(setPublicSchedulable).toHaveBeenCalledWith(7, false)
    expect(showSuccess).toHaveBeenCalled()
  })

  it('persists the account master switch immediately', async () => {
    listSmartScheduleMemberships.mockResolvedValue([])
    setSchedulable.mockResolvedValue({ ...account, schedulable: false })
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-testid="account-schedule-master"]').trigger('click')
    await flushPromises()
    expect(setSchedulable).toHaveBeenCalledWith(7, false)
  })

  it('only offers this account platform, plus AG for OpenAI', async () => {
    listSmartScheduleMemberships.mockResolvedValue([])
    const anthropic = mountPanel()
    await flushPromises()
    expect(anthropic.find('[data-testid="account-schedule-platform-anthropic"]').exists()).toBe(true)
    expect(anthropic.find('[data-testid="account-schedule-platform-openai"]').exists()).toBe(false)
    expect(anthropic.find('[data-testid="account-schedule-platform-antigravity"]').exists()).toBe(false)
    anthropic.unmount()

    const openai = mountPanel({ platform: 'openai' })
    await flushPromises()
    expect(openai.find('[data-testid="account-schedule-platform-openai"]').exists()).toBe(true)
    expect(openai.find('[data-testid="account-schedule-platform-antigravity"]').exists()).toBe(true)
    expect(openai.find('[data-testid="account-schedule-platform-gemini"]').exists()).toBe(false)
  })

  it('batch admission only targets selected members', async () => {
    listSmartScheduleMemberships.mockResolvedValue([
      { user_id: 16, email: 'a@x.com', deleted: false, platform: 'anthropic', enabled: true, paused: false },
      { user_id: 42, email: 'b@x.com', deleted: false, platform: 'anthropic', enabled: true, paused: false }
    ])
    setSmartScheduleAdmissionBatch.mockResolvedValue([])
    const wrapper = mountPanel()
    await flushPromises()
    const batch = wrapper.get('[data-testid="account-schedule-batch-admission"]')
    expect((batch.element as HTMLButtonElement).disabled).toBe(true)
    await wrapper.get('[data-testid="account-schedule-select-16"]').setValue(true)
    await batch.trigger('click')
    await flushPromises()
    expect(setSmartScheduleAdmissionBatch).toHaveBeenCalledWith(7, {
      platform: 'anthropic',
      user_ids: [16],
      state: 'paused'
    })
  })
})
