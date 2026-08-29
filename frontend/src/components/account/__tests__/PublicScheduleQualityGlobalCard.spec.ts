import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

const {
  getPublicScheduleQualitySettings,
  updatePublicScheduleQualitySettings,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getPublicScheduleQualitySettings: vi.fn(),
  updatePublicScheduleQualitySettings: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    settings: {
      getPublicScheduleQualitySettings,
      updatePublicScheduleQualitySettings
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError
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

import PublicScheduleQualityGlobalCard from '../PublicScheduleQualityGlobalCard.vue'

const ToggleStub = defineComponent({
  name: 'Toggle',
  props: { modelValue: { type: Boolean, default: false } },
  emits: ['update:modelValue'],
  template:
    '<button type="button" :data-on="modelValue" @click="$emit(\'update:modelValue\', !modelValue)"><slot /></button>'
})

const HelpTooltipStub = defineComponent({
  name: 'HelpTooltip',
  props: { content: { type: String, default: '' } },
  template: '<span />'
})

function mountCard() {
  return mount(PublicScheduleQualityGlobalCard, {
    global: {
      stubs: { Toggle: ToggleStub, HelpTooltip: HelpTooltipStub }
    }
  })
}

describe('PublicScheduleQualityGlobalCard', () => {
  beforeEach(() => {
    getPublicScheduleQualitySettings.mockReset()
    updatePublicScheduleQualitySettings.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    getPublicScheduleQualitySettings.mockResolvedValue({
      enabled: false,
      ttft_window_n: 10,
      success_window_n: 10,
      quality_max_p50_ttft_ms: 3000,
      quality_min_success_rate: 0.9,
      quality_max_p50_duration_ms: 80000,
      quality_max_slow_in_window: 2,
      quality_max_consecutive_slow: 2,
      quality_sched_window_n: 10,
      quality_sched_max_slow_in_window: 3,
      quality_sched_max_consecutive_slow: 2,
      cooldown_minutes: 15,
      soft_cooldown: false
    })
    updatePublicScheduleQualitySettings.mockResolvedValue({
      enabled: true,
      ttft_window_n: 8,
      success_window_n: 8,
      quality_max_p50_ttft_ms: 2500,
      quality_min_success_rate: 0.85,
      quality_max_p50_duration_ms: 60000,
      quality_max_slow_in_window: 4,
      quality_max_consecutive_slow: 3,
      quality_sched_window_n: 12,
      quality_sched_max_slow_in_window: 5,
      quality_sched_max_consecutive_slow: 2,
      cooldown_minutes: 20,
      soft_cooldown: true
    })
  })

  it('loads the same EvalQuality groups as the smart-schedule pool', async () => {
    const wrapper = mountCard()
    await flushPromises()

    expect(getPublicScheduleQualitySettings).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="public-quality-knob-strip"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="public-quality-top-bar"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="public-quality-threshold-group"]').text()).toContain(
      'admin.users.smartSchedule.thresholdMsGroup'
    )
    expect(wrapper.get('[data-test="public-quality-probe-group"]').text()).toContain(
      'admin.users.smartSchedule.probePhaseGroup'
    )
    expect(wrapper.get('[data-test="public-quality-sched-group"]').text()).toContain(
      'admin.users.smartSchedule.schedPhaseGroup'
    )
    expect(wrapper.get('[data-test="public-quality-ttft-n"]').element).toHaveProperty('value', '10')
    expect(wrapper.get('[data-test="public-quality-success-rate"]').element).toHaveProperty('value', '90')
    expect(wrapper.get('[data-test="public-quality-p50-duration"]').element).toHaveProperty('value', '80000')
    expect(wrapper.get('[data-test="public-quality-probe-k"]').element).toHaveProperty('value', '2')
    expect(wrapper.get('[data-test="public-quality-sched-k"]').element).toHaveProperty('value', '3')
    expect(wrapper.get('[data-test="public-quality-sched-c"]').element).toHaveProperty('value', '2')
  })

  it('saves threshold, probe, sched, and cooldown knobs', async () => {
    const wrapper = mountCard()
    await flushPromises()

    await wrapper.get('[data-test="public-quality-enabled"]').trigger('click')
    await wrapper.get('[data-test="public-quality-ttft-n"]').setValue('8')
    await wrapper.get('[data-test="public-quality-success-n"]').setValue('8')
    await wrapper.get('[data-test="public-quality-p50"]').setValue('2500')
    await wrapper.get('[data-test="public-quality-p50-duration"]').setValue('60000')
    await wrapper.get('[data-test="public-quality-success-rate"]').setValue('85')
    await wrapper.get('[data-test="public-quality-probe-k"]').setValue('4')
    await wrapper.get('[data-test="public-quality-probe-c"]').setValue('3')
    await wrapper.get('[data-test="public-quality-sched-n"]').setValue('12')
    await wrapper.get('[data-test="public-quality-sched-k"]').setValue('5')
    await wrapper.get('[data-test="public-quality-sched-c"]').setValue('2')
    await wrapper.get('[data-test="public-quality-cooldown"]').setValue('20')
    await wrapper.get('[data-test="public-quality-soft"]').trigger('click')
    await wrapper.get('[data-test="public-quality-save"]').trigger('click')
    await flushPromises()

    expect(updatePublicScheduleQualitySettings).toHaveBeenCalledWith({
      enabled: true,
      ttft_window_n: 8,
      success_window_n: 8,
      quality_max_p50_ttft_ms: 2500,
      quality_min_success_rate: 0.85,
      quality_max_p50_duration_ms: 60000,
      quality_max_slow_in_window: 4,
      quality_max_consecutive_slow: 3,
      quality_sched_window_n: 12,
      quality_sched_max_slow_in_window: 5,
      quality_sched_max_consecutive_slow: 2,
      cooldown_minutes: 20,
      soft_cooldown: true
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.publicQuality.saved')
  })
})
