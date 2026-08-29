import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpenAI7dCycleDialog from '../OpenAI7dCycleDialog.vue'
import type { Account } from '@/types'

const { getOpenAI7dCycleHistory } = vi.hoisted(() => ({
  getOpenAI7dCycleHistory: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getOpenAI7dCycleHistory
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
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

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    props: ['data', 'options'],
    template:
      '<div data-test="line-chart">{{ data.datasets.map((dataset) => dataset.label + ":" + dataset.data.join("/")).join("|") }}</div>'
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

function makeAccount(): Account {
  return {
    id: 88,
    name: 'oauth',
    platform: 'openai',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null
  } as Account
}

describe('OpenAI7dCycleDialog', () => {
  beforeEach(() => {
    getOpenAI7dCycleHistory.mockReset()
  })

  it('renders chart and table rows for current plus closed cycles', async () => {
    getOpenAI7dCycleHistory.mockResolvedValue({
      items: [
        {
          current: true,
          window_start: '2026-08-21T00:00:00Z',
          window_end: '2026-08-28T00:00:00Z',
          litellm_cost: 4.5,
          account_cost: 1.2,
          user_cost: 0.8,
          used_percent: 12,
          requests: 3,
          tokens: 300
        },
        {
          current: false,
          window_start: '2026-08-14T00:00:00Z',
          window_end: '2026-08-21T00:00:00Z',
          litellm_cost: 9.25,
          account_cost: 3.1,
          user_cost: 2.2,
          used_percent: 88,
          requests: 10,
          tokens: 1000
        }
      ]
    })

    const wrapper = mount(OpenAI7dCycleDialog, {
      props: {
        show: true,
        account: makeAccount()
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<div><slot /><slot name="footer" /></div>'
          },
          EmptyState: true,
          LoadingSpinner: true
        }
      }
    })

    await flushPromises()

    expect(getOpenAI7dCycleHistory).toHaveBeenCalledWith(88)
    expect(wrapper.get('[data-test="line-chart"]').text()).toContain('4.5')
    expect(wrapper.get('[data-test="line-chart"]').text()).toContain('9.25')
    expect(wrapper.get('[data-test="openai-7d-cycle-current"]').text()).toContain('$4.50')
    expect(wrapper.get('[data-test="openai-7d-cycle-closed"]').text()).toContain('$9.25')
    expect(wrapper.get('[data-test="openai-7d-cycle-closed"]').text()).toContain('$3.10')
  })
})
