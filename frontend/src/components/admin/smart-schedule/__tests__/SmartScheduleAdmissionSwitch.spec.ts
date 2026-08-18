import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SmartScheduleAdmissionSwitch from '../SmartScheduleAdmissionSwitch.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('SmartScheduleAdmissionSwitch', () => {
  it('opens a menu and emits the selected live state', async () => {
    const w = mount(SmartScheduleAdmissionSwitch, {
      props: { admission: 'will_cool' },
      attachTo: document.body
    })
    await w.get('[data-testid="smart-schedule-admission-switch"]').trigger('click')
    await flushPromises()
    expect(document.querySelector('[data-testid="smart-schedule-admission-paused"]')).toBeTruthy()
    const resumed = document.querySelector('[data-testid="smart-schedule-admission-resumed"]') as HTMLButtonElement
    expect(resumed).toBeTruthy()
    resumed.click()
    await flushPromises()
    expect(w.emitted('select')?.[0]).toEqual(['resumed'])
    w.unmount()
  })

  it('disables the trigger for unsaved preview', () => {
    const w = mount(SmartScheduleAdmissionSwitch, {
      props: { admission: 'unsaved_preview', disabled: true }
    })
    expect(w.get('[data-testid="smart-schedule-admission-switch"]').attributes('disabled')).toBeDefined()
  })
})
