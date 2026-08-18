import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import AccountStabilityDialog from '../AccountStabilityDialog.vue'
import type { Account } from '@/types'
import {
  STABILITY_P95_CLIP_FACTOR,
  STABILITY_SHOW_P95_STORAGE_KEY,
  STABILITY_TTFT_HEADROOM
} from '@/utils/accountStabilityChart'

const {
  getQualityHistory,
  getQualityHardClose,
  getBatchQualityStats,
  updateQualityHardClose,
  recoverState,
  getQualityHardCloseSettings,
  updateQualityHardCloseSettings,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getQualityHistory: vi.fn(),
  getQualityHardClose: vi.fn(),
  getBatchQualityStats: vi.fn(),
  updateQualityHardClose: vi.fn(),
  recoverState: vi.fn(),
  getQualityHardCloseSettings: vi.fn(),
  updateQualityHardCloseSettings: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getQualityHistory,
      getQualityHardClose,
      getBatchQualityStats,
      updateQualityHardClose,
      recoverState
    },
    settings: {
      getQualityHardCloseSettings,
      updateQualityHardCloseSettings
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError
  })
}))

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template:
      '<div data-test="line-chart">{{ data.labels.join(",") }}|{{ data.datasets.map((dataset) => dataset.label + ":" + dataset.data.join("/")).join("|") }}|ymax:{{ options?.scales?.yMs?.max ?? "" }}</div>'
  }
}))

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  CategoryScale: {},
  LinearScale: {},
  PointElement: {},
  LineElement: {},
  Tooltip: {},
  Legend: {},
  Filler: {}
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'admin.accounts.stability.title') return `stability:${params?.name ?? ''}`
        if (key === 'admin.accounts.stability.pauseBanner') return `paused:${params?.time ?? ''}`
        if (key === 'admin.accounts.stability.bridgeSamples') {
          return `bridgeSamples:${params?.success ?? ''}/${params?.error ?? ''}`
        }
        if (key === 'admin.accounts.stability.failoverSamples') {
          return `failoverSamples:${params?.terminal ?? ''}/${params?.failover ?? ''}`
        }
        return key
      }
    })
  }
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: { type: Boolean, default: false },
    title: { type: String, default: '' }
  },
  emits: ['close'],
  setup(props, { slots }) {
    return () =>
      props.show
        ? h('div', { 'data-test': 'stability-dialog' }, [
            h('h3', props.title),
            slots.default?.(),
            slots.footer?.()
          ])
        : null
  }
})

const ToggleStub = defineComponent({
  props: {
    modelValue: { type: Boolean, default: false }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        type: 'checkbox',
        class: 'toggle-stub',
        checked: props.modelValue,
        onChange: (event: Event) => {
          emit('update:modelValue', (event.target as HTMLInputElement).checked)
        }
      })
  }
})

function makeAccount(overrides: Partial<Account> = {}): Account {
  return {
    id: 12,
    name: 'acct-a',
    platform: 'anthropic',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-08-14T00:00:00Z',
    updated_at: '2026-08-14T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  }
}

const defaultHardClose = {
  overlay: {
    enabled: false,
    use_global: true,
    max_p50_ttft_ms: null,
    min_success_rate: null,
    pause_minutes: null,
    min_success_samples: null,
    min_ttft_samples: null,
    condition: null
  },
  resolved: {
    enabled: false,
    max_p50_ttft_ms: 3000,
    min_success_rate: 0.9,
    pause_minutes: 30,
    min_success_samples: 20,
    min_ttft_samples: 10,
    condition: 'or'
  },
  global_enabled: false
}

function mountDialog(account: Account | null = makeAccount()) {
  return mount(AccountStabilityDialog, {
    props: { show: true, account },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Toggle: ToggleStub,
        EmptyState: {
          props: ['title', 'description'],
          template: '<div data-test="stability-empty">{{ title }} {{ description }}</div>'
        },
        LoadingSpinner: true
      }
    }
  })
}

const historyWithSkewedP95 = {
  items: [
    {
      captured_at: '2026-08-14T10:00:00Z',
      window_seconds: 900,
      success_count: 10,
      error_count: 2,
      success_rate: 0.833,
      avg_ttft_ms: 400,
      p50_ttft_ms: 300,
      p95_ttft_ms: 9000,
      max_ttft_ms: 12000,
      ttft_samples: 10
    },
    {
      captured_at: '2026-08-14T10:05:00Z',
      window_seconds: 900,
      success_count: 12,
      error_count: 1,
      success_rate: 0.923,
      avg_ttft_ms: 350,
      p50_ttft_ms: 280,
      p95_ttft_ms: 8000,
      max_ttft_ms: 11000,
      ttft_samples: 12
    }
  ],
  from: '2026-08-13T10:00:00Z',
  to: '2026-08-14T10:05:00Z'
}

describe('AccountStabilityDialog', () => {
  beforeEach(() => {
    localStorage.removeItem(STABILITY_SHOW_P95_STORAGE_KEY)
    getQualityHistory.mockReset()
    getQualityHardClose.mockReset()
    getBatchQualityStats.mockReset()
    updateQualityHardClose.mockReset()
    recoverState.mockReset()
    getQualityHardCloseSettings.mockReset()
    updateQualityHardCloseSettings.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    getQualityHistory.mockResolvedValue({ items: [], from: '2026-08-13T00:00:00Z', to: '2026-08-14T00:00:00Z' })
    getQualityHardClose.mockResolvedValue({ ...defaultHardClose })
    getBatchQualityStats.mockResolvedValue({ stats: {} })
    updateQualityHardClose.mockResolvedValue({
      ...defaultHardClose,
      overlay: { ...defaultHardClose.overlay, enabled: true, use_global: false }
    })
    getQualityHardCloseSettings.mockResolvedValue({
      enabled: false,
      max_p50_ttft_ms: 3000,
      min_success_rate: 0.9,
      pause_minutes: 30,
      min_success_samples: 20,
      min_ttft_samples: 10,
      condition: 'or',
      schedule_use_failover_error_rate: false
    })
    updateQualityHardCloseSettings.mockResolvedValue({
      enabled: false,
      max_p50_ttft_ms: 2500,
      min_success_rate: 0.85,
      pause_minutes: 30,
      min_success_samples: 20,
      min_ttft_samples: 10,
      condition: 'or'
    })
  })

  it('shows empty chart copy when history has no points', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(getQualityHistory).toHaveBeenCalledWith(12)
    expect(getQualityHardClose).toHaveBeenCalledWith(12)
    expect(getBatchQualityStats).toHaveBeenCalledWith([12])
    expect(wrapper.get('[data-test="stability-empty"]').text()).toContain('admin.accounts.stability.noData')
    expect(wrapper.get('[data-test="stability-bridge-rate-value"]').text()).toBe('—')
    expect(wrapper.get('[data-test="stability-bridge-empty"]').text()).toContain('admin.accounts.stability.bridgeEmpty')
    expect(wrapper.find('[data-test="line-chart"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="global-disabled-hint"]').exists()).toBe(true)
  })

  it('plots overlapping quality-history window points on the chart', async () => {
    getQualityHistory.mockResolvedValue(historyWithSkewedP95)

    const wrapper = mountDialog()
    await flushPromises()

    expect(getQualityHistory).toHaveBeenCalledWith(12)
    expect(wrapper.find('[data-test="stability-empty"]').exists()).toBe(false)
    const chart = wrapper.get('[data-test="line-chart"]').text()
    expect(chart).toContain('300/280')
    expect(chart).not.toContain('9000/8000')
    expect(chart).toContain('83.3/92.3')
    expect(chart).toContain(`ymax:${Math.ceil(300 * STABILITY_TTFT_HEADROOM)}`)
    expect(wrapper.get('[data-test="stability-show-p95"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('admin.accounts.stability.chartHint')
  })

  it('hides p95 by default and rescales the left axis to p50', async () => {
    getQualityHistory.mockResolvedValue(historyWithSkewedP95)

    const wrapper = mountDialog()
    await flushPromises()

    const checkbox = wrapper.get<HTMLInputElement>('[data-test="stability-show-p95"]')
    expect(checkbox.element.checked).toBe(false)
    expect(wrapper.get('[data-test="line-chart"]').text()).toContain(`ymax:${Math.ceil(300 * STABILITY_TTFT_HEADROOM)}`)
    expect(wrapper.find('[data-test="stability-p95-clipped"]').exists()).toBe(false)
  })

  it('shows clipped p95 on a p50-focused axis and persists the toggle', async () => {
    getQualityHistory.mockResolvedValue(historyWithSkewedP95)

    const wrapper = mountDialog()
    await flushPromises()

    await wrapper.get('[data-test="stability-show-p95"]').setValue(true)
    await flushPromises()

    const chart = wrapper.get('[data-test="line-chart"]').text()
    expect(chart).toContain('300/280')
    expect(chart).toContain(`${Math.ceil(300 * STABILITY_P95_CLIP_FACTOR)}/${Math.ceil(300 * STABILITY_P95_CLIP_FACTOR)}`)
    expect(chart).not.toContain('9000/8000')
    expect(chart).toContain(`ymax:${Math.ceil(300 * STABILITY_P95_CLIP_FACTOR)}`)
    expect(wrapper.get('[data-test="stability-p95-clipped"]').text()).toContain('admin.accounts.stability.p95ClippedHint')
    expect(localStorage.getItem(STABILITY_SHOW_P95_STORAGE_KEY)).toBe('1')

    wrapper.unmount()
    const reopened = mountDialog()
    await flushPromises()
    expect(reopened.get<HTMLInputElement>('[data-test="stability-show-p95"]').element.checked).toBe(true)
    expect(reopened.get('[data-test="line-chart"]').text()).toContain(`ymax:${Math.ceil(300 * STABILITY_P95_CLIP_FACTOR)}`)
    reopened.unmount()
  })

  it('saves the overlay as a top-level payload and converts percent to 0-1', async () => {
    getQualityHardClose.mockResolvedValue({
      ...defaultHardClose,
      overlay: {
        ...defaultHardClose.overlay,
        use_global: false,
        max_p50_ttft_ms: 2500,
        min_success_rate: 0.9,
        pause_minutes: 30,
        min_success_samples: 20,
        min_ttft_samples: 10,
        condition: 'or'
      },
      global_enabled: true
    })

    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.findAll('.toggle-stub').length).toBe(2)
    await wrapper.get('[data-test="stability-hard-close-enabled"]').setValue(true)

    const percentInput = wrapper.find('input[type="number"][step="0.1"]')
    await percentInput.setValue('85')
    await wrapper.get('[data-test="stability-save"]').trigger('click')
    await flushPromises()

    expect(updateQualityHardClose).toHaveBeenCalledTimes(1)
    const [id, payload] = updateQualityHardClose.mock.calls[0]
    expect(id).toBe(12)
    expect(payload).toEqual({
      enabled: true,
      use_global: false,
      max_p50_ttft_ms: 2500,
      min_success_rate: 0.85,
      pause_minutes: 30,
      min_success_samples: 20,
      min_ttft_samples: 10,
      condition: 'or'
    })
    expect(payload.overlay).toBeUndefined()
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.stability.saveSuccess')
  })

  it('shows the quality-pause banner when reason is active', async () => {
    const wrapper = mountDialog(
      makeAccount({
        temp_unschedulable_until: '2099-08-14T16:30:00Z',
        temp_unschedulable_reason: 'quality_hard_close:p50=3200,success=0.82'
      })
    )
    await flushPromises()

    expect(wrapper.get('[data-test="quality-pause-banner"]').text()).toContain('paused:')
    expect(wrapper.get('[data-test="stability-resume-now"]').exists()).toBe(true)
  })

  it('resumes a quality pause immediately without waiting for the timer', async () => {
    const paused = makeAccount({
      temp_unschedulable_until: '2099-08-14T16:30:00Z',
      temp_unschedulable_reason: 'quality_hard_close:p50=3200,success=0.82'
    })
    const recovered = makeAccount({
      temp_unschedulable_until: null,
      temp_unschedulable_reason: null
    })
    recoverState.mockResolvedValue(recovered)

    const wrapper = mountDialog(paused)
    await flushPromises()
    await wrapper.get('[data-test="stability-resume-now"]').trigger('click')
    await flushPromises()

    expect(recoverState).toHaveBeenCalledWith(12)
    expect(wrapper.emitted('recovered')?.[0]?.[0]).toEqual(recovered)
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.stability.resumeSuccess')
  })

  it('applies the shared template into the form without saving the account', async () => {
    getQualityHardCloseSettings.mockResolvedValue({
      enabled: true,
      max_p50_ttft_ms: 1800,
      min_success_rate: 0.95,
      pause_minutes: 15,
      min_success_samples: 8,
      min_ttft_samples: 6,
      condition: 'and'
    })

    const wrapper = mountDialog()
    await flushPromises()

    await wrapper.get('[data-test="stability-apply-template"]').trigger('click')
    await flushPromises()

    expect(getQualityHardCloseSettings).toHaveBeenCalled()
    expect(updateQualityHardClose).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.stability.applyTemplateSuccess')

    await wrapper.get('[data-test="stability-save"]').trigger('click')
    await flushPromises()
    expect(updateQualityHardClose.mock.calls[0][1]).toEqual({
      enabled: false,
      use_global: false,
      max_p50_ttft_ms: 1800,
      min_success_rate: 0.95,
      pause_minutes: 15,
      min_success_samples: 8,
      min_ttft_samples: 6,
      condition: 'and'
    })
  })

  it('saves the current form as the only shared template without flipping the master switch', async () => {
    getQualityHardClose.mockResolvedValue({
      ...defaultHardClose,
      overlay: {
        ...defaultHardClose.overlay,
        use_global: false,
        max_p50_ttft_ms: 2500,
        min_success_rate: 0.85,
        pause_minutes: 40,
        min_success_samples: 12,
        min_ttft_samples: 9,
        condition: 'or'
      }
    })

    const wrapper = mountDialog()
    await flushPromises()

    await wrapper.get('[data-test="stability-save-template"]').trigger('click')
    await flushPromises()

    expect(updateQualityHardCloseSettings).toHaveBeenCalledWith({
      enabled: false,
      max_p50_ttft_ms: 2500,
      min_success_rate: 0.85,
      pause_minutes: 40,
      min_success_samples: 12,
      min_ttft_samples: 9,
      condition: 'or',
      schedule_use_failover_error_rate: false
    })
    expect(updateQualityHardClose).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.stability.saveTemplateSuccess')
  })

  it('shows the live 15-minute bridge error rate and sample counts', async () => {
    getBatchQualityStats.mockResolvedValue({
      stats: {
        '12': {
          window_seconds: 900,
          success_count: 10,
          error_count: 1,
          success_rate: 0.91,
          bridge_success_count: 4,
          bridge_error_count: 6,
          bridge_error_rate: 0.6,
          avg_ttft_ms: 400,
          p50_ttft_ms: 300,
          p95_ttft_ms: 900,
          max_ttft_ms: 1200,
          ttft_samples: 10
        }
      }
    })

    const wrapper = mountDialog()
    await flushPromises()

    expect(getBatchQualityStats).toHaveBeenCalledWith([12])
    expect(wrapper.get('[data-test="stability-bridge-rate"]').text()).toContain('admin.accounts.stability.bridgeTitle')
    expect(wrapper.get('[data-test="stability-bridge-rate-value"]').text()).toBe('60.0%')
    expect(wrapper.get('[data-test="stability-bridge-samples"]').text()).toBe('bridgeSamples:4/6')
    expect(wrapper.find('[data-test="stability-bridge-empty"]').exists()).toBe(false)
  })

  it('does not render 0% when the live window has no bridge samples', async () => {
    getBatchQualityStats.mockResolvedValue({
      stats: {
        '12': {
          window_seconds: 900,
          success_count: 10,
          error_count: 1,
          success_rate: 0.91,
          bridge_success_count: 0,
          bridge_error_count: 0,
          bridge_error_rate: 0,
          avg_ttft_ms: 400,
          p50_ttft_ms: 300,
          p95_ttft_ms: 900,
          max_ttft_ms: 1200,
          ttft_samples: 10
        }
      }
    })

    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.get('[data-test="stability-bridge-rate-value"]').text()).toBe('—')
    expect(wrapper.get('[data-test="stability-bridge-empty"]').text()).toContain('admin.accounts.stability.bridgeEmpty')
    expect(wrapper.text()).not.toContain('0.0%')
    expect(wrapper.find('[data-test="stability-bridge-samples"]').exists()).toBe(false)
  })

  it('shows both account error rates and persists the failover scheduling toggle', async () => {
    getBatchQualityStats.mockResolvedValue({
      stats: {
        '12': {
          window_seconds: 900,
          success_count: 90,
          error_count: 5,
          success_rate: 90 / 95,
          terminal_error_count: 5,
          terminal_error_rate: 5 / 95,
          failover_error_count: 20,
          failover_error_rate: 20 / 110,
          avg_ttft_ms: 400,
          p50_ttft_ms: 300,
          ttft_samples: 10
        }
      }
    })
    updateQualityHardCloseSettings.mockResolvedValue({
      enabled: false,
      max_p50_ttft_ms: 3000,
      min_success_rate: 0.9,
      pause_minutes: 30,
      min_success_samples: 20,
      min_ttft_samples: 10,
      condition: 'or',
      schedule_use_failover_error_rate: true
    })

    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.get('[data-test="stability-error-calibers"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="stability-terminal-rate"]').text()).toBe('5.3%')
    expect(wrapper.get('[data-test="stability-failover-rate"]').text()).toBe('18.2%')
    expect(wrapper.get('[data-test="stability-caliber-samples"]').text()).toContain('5')
    expect(wrapper.get('[data-test="stability-caliber-samples"]').text()).toContain('20')

    await wrapper.get('[data-test="stability-failover-toggle"]').setValue(true)
    await flushPromises()

    expect(updateQualityHardCloseSettings).toHaveBeenCalledWith(
      expect.objectContaining({ schedule_use_failover_error_rate: true })
    )
  })
})
