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
    expect(document.querySelector('[data-testid="smart-schedule-admission-probing"]')).toBeTruthy()
    expect(document.querySelector('[data-testid="smart-schedule-admission-unpause"]')).toBeNull()
    const resumed = document.querySelector('[data-testid="smart-schedule-admission-resumed"]') as HTMLButtonElement
    expect(resumed).toBeTruthy()
    resumed.click()
    await flushPromises()
    expect(w.emitted('select')?.[0]).toEqual(['resumed'])
    w.unmount()
  })

  it('keeps the paused checkmark when the chip is account-level stopped', async () => {
    const w = mount(SmartScheduleAdmissionSwitch, {
      props: { admission: 'stopped', paused: true },
      attachTo: document.body
    })
    await w.get('[data-testid="smart-schedule-admission-switch"]').trigger('click')
    await flushPromises()
    const paused = document.querySelector('[data-testid="smart-schedule-admission-paused"]') as HTMLButtonElement
    expect(paused?.textContent).toContain('✓')
    w.unmount()
  })

  it('lets selectable pick probing without waiting for cooldown', async () => {
    const w = mount(SmartScheduleAdmissionSwitch, {
      props: { admission: 'selectable' },
      attachTo: document.body
    })
    await w.get('[data-testid="smart-schedule-admission-switch"]').trigger('click')
    await flushPromises()
    const probing = document.querySelector('[data-testid="smart-schedule-admission-probing"]') as HTMLButtonElement
    expect(probing).toBeTruthy()
    probing.click()
    await flushPromises()
    expect(w.emitted('select')?.[0]).toEqual(['probing'])
    w.unmount()
  })

  it('disables the trigger for unsaved preview', () => {
    const w = mount(SmartScheduleAdmissionSwitch, {
      props: { admission: 'unsaved_preview', disabled: true }
    })
    expect(w.get('[data-testid="smart-schedule-admission-switch"]').attributes('disabled')).toBeDefined()
  })
})
