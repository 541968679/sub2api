import { describe, expect, it } from 'vitest'
import { isCurrentlySchedulingAccount } from '../useUserSmartScheduleEditor'

describe('isCurrentlySchedulingAccount', () => {
  it('accepts active schedulable accounts', () => {
    expect(isCurrentlySchedulingAccount({ status: 'active', schedulable: true })).toBe(true)
  })

  it('rejects paused, unschedulable, or temporarily blocked accounts', () => {
    expect(isCurrentlySchedulingAccount({ status: 'inactive', schedulable: true })).toBe(false)
    expect(isCurrentlySchedulingAccount({ status: 'active', schedulable: false })).toBe(false)
    expect(
      isCurrentlySchedulingAccount({
        status: 'active',
        schedulable: true,
        temp_unschedulable_until: new Date(Date.now() + 60_000).toISOString()
      })
    ).toBe(false)
    expect(
      isCurrentlySchedulingAccount({
        status: 'active',
        schedulable: true,
        rate_limit_reset_at: new Date(Date.now() + 60_000).toISOString()
      })
    ).toBe(false)
  })
})
