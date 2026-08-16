import { describe, expect, it } from 'vitest'
import { isCurrentlySchedulingAccount } from '../useUserSmartScheduleEditor'
import { resolvePairCap } from '../smartSchedulePoolAdmission'

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

describe('effective pair cap display', () => {
  it('does not fall back to account-wide concurrency', () => {
    expect(resolvePairCap(null)).toBeNull()
    expect(resolvePairCap(0)).toBeNull()
    expect(resolvePairCap(4)).toBe(4)
  })
})
