import { describe, expect, it } from 'vitest'
import {
  buildAdminUserListRowColumns,
  formatAdminUserBurnRateAmount,
  formatAdminUserBurnRateDisplay,
  formatAdminUserBurnRateTotal,
  getAdminUserStatusToggleConfirm,
  getAdminUserToggleStatusTarget,
  pickBatchUserStat,
  smartScheduleSummaryFromDrafts,
  sumDisplayedAdminUserBurnRates
} from '../adminUserListRow'

describe('smartScheduleSummaryFromDrafts', () => {
  it('only lists enabled platforms that have pool members', () => {
    const summary = smartScheduleSummaryFromDrafts(['anthropic', 'openai'], {
      anthropic: { enabled: true, accounts: [{}, {}] },
      openai: { enabled: true, accounts: [] }
    })
    expect(summary.enabled_platforms).toEqual(['anthropic'])
    expect(summary.pool_counts).toEqual({ anthropic: 2, openai: 0 })
  })
})

describe('buildAdminUserListRowColumns', () => {
  it('omits username and inserts burn rate after balance', () => {
    const keys = buildAdminUserListRowColumns((key) => key).map((col) => col.key)
    expect(keys).not.toContain('username')
    expect(keys).toContain('burn_rate')
    expect(keys.indexOf('burn_rate')).toBe(keys.indexOf('balance') + 1)
    expect(keys.at(-1)).toBe('actions')
    expect(keys.indexOf('schedule_pnl')).toBe(keys.indexOf('smart_schedule') + 1)
    expect(keys).toContain('quality_ttft')
    expect(keys).not.toContain('quality_success_rate')
    expect(keys.indexOf('quality_ttft')).toBe(keys.indexOf('schedule_pnl') + 1)
  })

  it('maps toggle status the same way as UsersView', () => {
    expect(getAdminUserToggleStatusTarget('active')).toBe('disabled')
    expect(getAdminUserToggleStatusTarget('disabled')).toBe('active')
    expect(getAdminUserToggleStatusTarget('pending_approval')).toBe('active')
  })

  it('asks to confirm disable, enable, and approve with the user email', () => {
    const translate = (key: string, params?: Record<string, unknown>) =>
      params?.email ? `${key}:${params.email}` : key
    expect(getAdminUserStatusToggleConfirm({ status: 'active', email: 'a@b.c' }, translate)).toEqual({
      title: 'admin.users.disableUser',
      message: 'admin.users.disableConfirm:a@b.c',
      confirmText: 'admin.users.disable',
      danger: true
    })
    expect(getAdminUserStatusToggleConfirm({ status: 'disabled', email: 'a@b.c' }, translate)).toEqual({
      title: 'admin.users.enableUser',
      message: 'admin.users.enableConfirm:a@b.c',
      confirmText: 'admin.users.enable',
      danger: false
    })
    expect(getAdminUserStatusToggleConfirm({ status: 'pending_approval', email: 'a@b.c' }, translate).title).toBe(
      'admin.users.approveUser'
    )
  })
})

describe('admin user burn-rate formatter', () => {
  it('matches UsersView $/h and $/min display', () => {
    expect(formatAdminUserBurnRateAmount(1.2, 'hour')).toBe(1.2)
    expect(formatAdminUserBurnRateAmount(1.2, 'minute')).toBe(0.02)
    expect(formatAdminUserBurnRateDisplay(1.2, 'hour')).toBe('$1.20/h')
    expect(formatAdminUserBurnRateDisplay(1.2, 'minute')).toBe('$0.02/min')
    expect(formatAdminUserBurnRateDisplay(0, 'hour')).toBe('$0.00/h')
  })

  it('adds the rounded amounts already shown on the rows', () => {
    const stats = {
      '1': { burn_rate_per_hour: 1.2 },
      '2': { burn_rate_per_hour: 0.3 }
    }
    expect(sumDisplayedAdminUserBurnRates([1, 2, 3], stats, 'hour')).toBe(1.5)
    expect(formatAdminUserBurnRateTotal(1.5, 'hour')).toBe('$1.50/h')
    expect(sumDisplayedAdminUserBurnRates([1, 2], stats, 'minute')).toBe(0.03)
    expect(formatAdminUserBurnRateTotal(0.03, 'minute')).toBe('$0.03/min')
  })

  it('picks batch stats by string user id', () => {
    expect(pickBatchUserStat({ '99': { burn_rate_per_hour: 1.2 } }, 99)).toEqual({
      burn_rate_per_hour: 1.2
    })
    expect(pickBatchUserStat({ '99': { burn_rate_per_hour: 1.2 } }, 1)).toBeNull()
  })
})
