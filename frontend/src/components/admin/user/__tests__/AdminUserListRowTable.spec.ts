import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AdminUserListRowTable from '../AdminUserListRowTable.vue'
import type { AdminUser } from '@/types'
import type { AccountQualityStats } from '@/api/admin/accounts'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const user: AdminUser = {
  id: 99,
  username: 'header-user',
  email: 'header@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-17T00:00:00Z',
  updated_at: '2026-04-17T00:00:00Z',
  notes: '',
  last_active_at: '2026-04-16T02:00:00Z',
  last_used_at: '2026-04-17T02:00:00Z',
  current_concurrency: 0
}

const stats: AccountQualityStats = {
  window_n: 20,
  account_quality_window_n: 20,
  success_count: 12,
  error_count: 1,
  success_rate: 0.92,
  failover_error_count: 2,
  failover_error_rate: 0.1,
  avg_ttft_ms: 400,
  p50_ttft_ms: 320,
  p95_ttft_ms: 800,
  max_ttft_ms: 1100,
  ttft_samples: 12
}

describe('AdminUserListRowTable user quality cell', () => {
  it('renders one combined clickable user quality cell and emits open-user-quality', async () => {
    const wrapper = mount(AdminUserListRowTable, {
      props: {
        user,
        groups: [],
        qualityStats: stats,
        windowN: 20
      },
      global: {
        stubs: {
          DataTable: {
            props: ['columns', 'data'],
            template: `
              <div>
                <div data-test="keys">{{ columns.map(col => col.key).join(',') }}</div>
                <slot name="header-quality_ttft" :column="columns.find(col => col.key === 'quality_ttft')" />
                <slot name="cell-quality_ttft" :row="data[0]" />
              </div>
            `
          },
          HelpTooltip: true,
          Icon: true,
          GroupBadge: true,
          UserBurnRateCell: true,
          UserConcurrencyCell: true,
          UserSchedulePnlCell: true,
          AdminUserListRowActions: true
        }
      }
    })

    expect(wrapper.get('[data-test="keys"]').text().split(',')).toContain('quality_ttft')
    expect(wrapper.get('[data-test="keys"]').text().split(',')).not.toContain('quality_success_rate')
    const button = wrapper.get('[data-test="user-quality-cell-button"]')
    expect(button.text()).toContain('320ms')
    expect(button.text()).toContain('92.0%')
    expect(button.get('[data-test="account-quality-failover-rate"]').text()).toContain('90.0%')
    expect(button.get('[data-test="account-quality-window-counts"]').exists()).toBe(true)
    await button.trigger('click')
    expect(wrapper.emitted('open-user-quality')).toHaveLength(1)
  })
})
