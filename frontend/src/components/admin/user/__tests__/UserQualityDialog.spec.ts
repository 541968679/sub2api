import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import UserQualityDialog from '../UserQualityDialog.vue'
import { STABILITY_SHOW_P95_STORAGE_KEY } from '@/utils/accountStabilityChart'

const {
  getQualityHistory,
  getBatchQualityStats,
  getQualityHardCloseSettings,
  getById,
  updateUser,
  accountsGetQualityHistory,
  showError,
  showSuccess
} = vi.hoisted(() => ({
  getQualityHistory: vi.fn(),
  getBatchQualityStats: vi.fn(),
  getQualityHardCloseSettings: vi.fn(),
  getById: vi.fn(),
  updateUser: vi.fn(),
  accountsGetQualityHistory: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getQualityHistory,
      getBatchQualityStats,
      getById,
      update: updateUser
    },
    settings: {
      getQualityHardCloseSettings
    },
    accounts: {
      getQualityHistory: accountsGetQualityHistory,
      getBatchQualityStats: vi.fn()
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template:
      '<div data-test="line-chart">{{ data.labels.join(",") }}|{{ data.datasets.map((dataset) => dataset.label + ":" + dataset.data.join("/")).join("|") }}</div>'
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
      t: (key: string, params?: Record<string, string | number>) => {
        if (key === 'admin.users.quality.title') return `user-quality:${params?.name ?? ''}`
        if (key === 'admin.users.quality.windowScope') return `window:${params?.n ?? ''}`
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
        ? h('div', { 'data-test': 'user-quality-host' }, [
            h('h3', props.title),
            slots.default?.()
          ])
        : null
  }
})

function mountDialog(userId: number | null = 42) {
  return mount(UserQualityDialog, {
    props: { show: true, userId, title: 'scoped@example.com' },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        EmptyState: {
          props: ['title', 'description'],
          template: '<div data-test="stability-empty">{{ title }} {{ description }}</div>'
        },
        LoadingSpinner: true
      }
    }
  })
}

describe('UserQualityDialog', () => {
  beforeEach(() => {
    localStorage.removeItem(STABILITY_SHOW_P95_STORAGE_KEY)
    getQualityHistory.mockReset()
    getBatchQualityStats.mockReset()
    getQualityHardCloseSettings.mockReset()
    getById.mockReset()
    updateUser.mockReset()
    accountsGetQualityHistory.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    getById.mockResolvedValue({ id: 42, quality_window_n: null })
    getQualityHistory.mockResolvedValue({
      items: [],
      from: '2026-08-20T00:00:00Z',
      to: '2026-08-21T00:00:00Z'
    })
    getBatchQualityStats.mockResolvedValue({
      stats: {
        '42': {
          window_n: 20,
          account_quality_window_n: 20,
          success_count: 10,
          error_count: 1,
          success_rate: 0.91,
          terminal_error_count: 1,
          terminal_error_rate: 0.09,
          failover_error_count: 3,
          failover_error_rate: 0.2,
          avg_ttft_ms: 400,
          p50_ttft_ms: 300,
          p95_ttft_ms: 900,
          max_ttft_ms: 1200,
          ttft_samples: 10
        }
      }
    })
    getQualityHardCloseSettings.mockResolvedValue({ account_quality_window_n: 20 })
  })

  it('loads user history and live last-N stats, never account quality-history', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.get('h3').text()).toBe('user-quality:scoped@example.com')
    expect(getQualityHistory).toHaveBeenCalledWith(42)
    expect(getBatchQualityStats).toHaveBeenCalledWith([42])
    expect(accountsGetQualityHistory).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="user-quality-window-scope"]').text()).toBe('window:20')
    expect((wrapper.get('[data-test="user-quality-window-n-inherit"]').element as HTMLInputElement).checked).toBe(true)
    expect(wrapper.get('[data-test="stability-failover-rate"]').text()).toContain('20.0%')
    expect(wrapper.get('[data-test="stability-empty"]').exists()).toBe(true)
  })

  it('renders the last-N snapshot chart', async () => {
    getQualityHistory.mockResolvedValue({
      items: [
        {
          captured_at: '2026-08-21T10:00:00Z',
          success_count: 8,
          error_count: 2,
          success_rate: 0.8,
          avg_ttft_ms: 400,
          p50_ttft_ms: 280,
          p95_ttft_ms: 800,
          max_ttft_ms: 1100,
          ttft_samples: 8
        }
      ],
      from: '2026-08-20T00:00:00Z',
      to: '2026-08-21T00:00:00Z'
    })
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.get('[data-test="stability-chart"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="stability-empty"]').exists()).toBe(false)
  })

  it('saves a per-user window N override', async () => {
    getById.mockResolvedValue({ id: 42, quality_window_n: 20 })
    updateUser.mockResolvedValue({ id: 42, quality_window_n: 8 })
    const wrapper = mountDialog()
    await flushPromises()

    await wrapper.get('[data-test="user-quality-window-n-inherit"]').setValue(false)
    await wrapper.get('[data-test="user-quality-window-n"]').setValue(8)
    await wrapper.get('[data-test="user-quality-window-n-save"]').trigger('click')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledWith(42, { quality_window_n: 8 })
    expect(showSuccess).toHaveBeenCalled()
    expect(getBatchQualityStats).toHaveBeenCalledTimes(2)
  })
})
