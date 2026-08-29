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

function mountCard() {
  return mount(PublicScheduleQualityGlobalCard, {
    global: {
      stubs: { Toggle: ToggleStub }
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
      cooldown_minutes: 15,
      soft_cooldown: false
    })
    updatePublicScheduleQualitySettings.mockResolvedValue({
      enabled: true,
      ttft_window_n: 8,
      success_window_n: 8,
      quality_max_p50_ttft_ms: 2500,
      quality_min_success_rate: 0.85,
      cooldown_minutes: 20,
      soft_cooldown: true
    })
  })

  it('loads site settings into the global card', async () => {
    const wrapper = mountCard()
    await flushPromises()

    expect(getPublicScheduleQualitySettings).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="public-schedule-quality-global"]').classes()).toContain('h-full')
    expect(wrapper.get('[data-test="public-quality-ttft-n"]').element).toHaveProperty('value', '10')
    expect(wrapper.get('[data-test="public-quality-success-rate"]').element).toHaveProperty('value', '90')
  })

  it('saves the global enable switch and knobs', async () => {
    const wrapper = mountCard()
    await flushPromises()

    await wrapper.get('[data-test="public-quality-enabled"]').trigger('click')
    await wrapper.get('[data-test="public-quality-ttft-n"]').setValue('8')
    await wrapper.get('[data-test="public-quality-success-n"]').setValue('8')
    await wrapper.get('[data-test="public-quality-p50"]').setValue('2500')
    await wrapper.get('[data-test="public-quality-success-rate"]').setValue('85')
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
      cooldown_minutes: 20,
      soft_cooldown: true
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.publicQuality.saved')
  })
})
