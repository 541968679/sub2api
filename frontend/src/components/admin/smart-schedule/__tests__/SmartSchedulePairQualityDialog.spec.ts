import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SmartSchedulePairQualityDialog from '../SmartSchedulePairQualityDialog.vue'

const apiMocks = vi.hoisted(() => ({
  getSmartSchedulePairQualityDetail: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getSmartSchedulePairQualityDetail: apiMocks.getSmartSchedulePairQualityDetail
    }
  }
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => value
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) => {
      if (params?.name) return `${key}:${params.name}`
      if (params) return `${key}:${params.ttft}/${params.ok}/${params.n}`
      return key
    }
  })
}))

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    props: ['show', 'title'],
    template: '<div v-if="show"><slot /></div>'
  }
}))

vi.mock('@/components/common/EmptyState.vue', () => ({
  default: {
    props: ['title', 'description'],
    template: '<div data-testid="smart-schedule-pair-quality-empty">{{ title }}</div>'
  }
}))

vi.mock('@/components/common/LoadingSpinner.vue', () => ({
  default: { template: '<div>loading</div>' }
}))

vi.mock('vue-chartjs', () => ({
  Line: { template: '<div data-testid="pair-quality-line" />' }
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

describe('SmartSchedulePairQualityDialog', () => {
  beforeEach(() => {
    apiMocks.getSmartSchedulePairQualityDetail.mockReset()
    apiMocks.getSmartSchedulePairQualityDetail.mockResolvedValue({
      current: { ttft_p50_ms: 120, success_rate: 0.9, ttft_samples: 8, ok_samples: 10, n: 10 },
      snapshots: [
        {
          captured_at: '2026-08-21T03:00:00.000Z',
          ttft_p50_ms: 110,
          success_rate: 0.92,
          ttft_samples: 7,
          ok_samples: 9,
          n: 10
        }
      ],
      events: [
        { at: '2026-08-21T02:00:00.000Z', type: 'cooldown_start' },
        { at: '2026-08-21T02:30:00.000Z', type: 'resumed' },
        { at: '2026-08-21T02:32:00.000Z', type: 'pinned' },
        { at: '2026-08-21T02:45:00.000Z', type: 'selectable' },
        { at: '2026-08-21T02:50:00.000Z', type: 'probe_enter' },
        { at: '2026-08-21T02:55:00.000Z', type: 'probe_graduate' }
      ]
    })
  })

  it('loads pair-quality detail instead of account quality-history', async () => {
    const w = mount(SmartSchedulePairQualityDialog, {
      props: {
        show: true,
        userId: 99,
        account: { id: 11, name: 'acc-11' } as any
      }
    })
    await flushPromises()
    expect(apiMocks.getSmartSchedulePairQualityDetail).toHaveBeenCalledWith(99, 11, undefined)
  })

  it('loads the tab platform pair-quality route when platform is set', async () => {
    const w = mount(SmartSchedulePairQualityDialog, {
      props: {
        show: true,
        userId: 99,
        platform: 'antigravity',
        account: { id: 11, name: 'acc-11' } as any
      }
    })
    await flushPromises()
    expect(apiMocks.getSmartSchedulePairQualityDetail).toHaveBeenCalledWith(99, 11, 'antigravity')
    expect(w.get('[data-testid="smart-schedule-pair-quality-dialog"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pair-quality-chart"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pair-quality-events"]').text()).toContain(
      'admin.users.smartSchedule.pairEventCooldownStart'
    )
    expect(w.get('[data-testid="smart-schedule-pair-quality-events"]').text()).toContain(
      'admin.users.smartSchedule.pairEventResumed'
    )
    expect(w.get('[data-testid="smart-schedule-pair-quality-events"]').text()).toContain(
      'admin.users.smartSchedule.pairEventPinned'
    )
    expect(w.get('[data-testid="smart-schedule-pair-quality-events"]').text()).toContain(
      'admin.users.smartSchedule.pairEventSelectable'
    )
    expect(w.get('[data-testid="smart-schedule-pair-quality-events"]').text()).toContain(
      'admin.users.smartSchedule.pairEventProbeEnter'
    )
    expect(w.get('[data-testid="smart-schedule-pair-quality-events"]').text()).toContain(
      'admin.users.smartSchedule.pairEventProbeGraduate'
    )
  })

  it('shows empty trend when the pair-quality endpoint is missing', async () => {
    apiMocks.getSmartSchedulePairQualityDetail.mockRejectedValueOnce(new Error('not implemented'))
    const w = mount(SmartSchedulePairQualityDialog, {
      props: {
        show: true,
        userId: 99,
        account: { id: 11, name: 'acc-11' } as any
      }
    })
    await flushPromises()
    expect(w.get('[data-testid="smart-schedule-pair-quality-empty"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-pair-quality-events-empty"]').exists()).toBe(true)
  })
})
