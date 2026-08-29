import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { PublicScheduleQualityView } from '@/api/admin/accounts'
import PublicScheduleQualityStateCell from '../PublicScheduleQualityStateCell.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

function view(partial: Partial<PublicScheduleQualityView> = {}): PublicScheduleQualityView {
  return {
    overlay: { enabled: false },
    resolved: {
      enabled: true,
      ttft_window_n: 20,
      success_window_n: 20,
      cooldown_minutes: 15,
      soft_cooldown: false
    },
    state: 'selectable',
    will_cool: false,
    ...partial
  }
}

describe('PublicScheduleQualityStateCell', () => {
  it('maps selectable+will_cool to the will-cool chip', () => {
    const wrapper = mount(PublicScheduleQualityStateCell, {
      props: { view: view({ will_cool: true, reason: 'p50' }) }
    })
    expect(wrapper.get('[data-testid="account-public-quality"]').attributes('data-state')).toBe('will_cool')
    expect(wrapper.text()).toContain('admin.accounts.publicQuality.stateWillCool')
    expect(wrapper.get('[data-testid="account-public-quality-reason"]').text()).toBe('p50')
    expect(wrapper.text()).not.toContain('admin.users.smartSchedule.admissionCooling')
  })

  it('shows account cooldown remaining and soft chip', () => {
    const until = new Date(Date.now() + 12 * 60_000).toISOString()
    const wrapper = mount(PublicScheduleQualityStateCell, {
      props: {
        view: view({
          state: 'cooling',
          until,
          reason: 'manual',
          resolved: {
            enabled: true,
            ttft_window_n: 20,
            success_window_n: 20,
            cooldown_minutes: 15,
            soft_cooldown: true
          }
        })
      }
    })
    expect(wrapper.get('[data-testid="account-public-quality"]').attributes('data-state')).toBe('cooling')
    expect(wrapper.text()).toContain('admin.accounts.publicQuality.stateCooling')
    expect(wrapper.get('[data-testid="account-public-quality-soft"]').text()).toBe(
      'admin.accounts.publicQuality.soft'
    )
    expect(wrapper.get('[data-testid="account-public-quality-remaining"]').text()).toContain(
      'admin.accounts.publicQuality.coolingRemaining'
    )
  })

  it('emits a live state from the shared switcher', async () => {
    const wrapper = mount(PublicScheduleQualityStateCell, {
      props: { view: view() },
      attachTo: document.body
    })
    await wrapper.get('[data-testid="smart-schedule-admission-switch"]').trigger('click')
    await flushPromises()
    const paused = document.querySelector('[data-testid="smart-schedule-admission-paused"]') as HTMLButtonElement
    expect(paused).toBeTruthy()
    expect(paused.textContent).toContain('admin.accounts.publicQuality.statePaused')
    paused.click()
    await flushPromises()
    expect(wrapper.emitted('select')?.[0]).toEqual(['paused'])
    wrapper.unmount()
  })
})
