import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  clearAffiliateReferralCode,
  clearAllAffiliateReferralCodes,
  clearOAuthAffiliateCode,
  loadAffiliateReferralCode,
  loadOAuthAffiliateCode,
  loadOAuthSalesCode,
  loadSalesReferralCode,
  oauthReferralPayload,
  prepareOAuthReferralCodes,
  resolveAffiliateReferralCode,
  storeAffiliateReferralCode,
  storeOAuthAffiliateCode,
  storeSalesReferralCode,
  syncExclusiveReferralCodes
} from '@/utils/oauthAffiliate'

describe('oauthAffiliate', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    vi.useRealTimers()
  })

  it('persists affiliate referral code across pages', () => {
    expect(resolveAffiliateReferralCode(' 5579J7CFG9PF ')).toBe('5579J7CFG9PF')
    expect(loadAffiliateReferralCode()).toBe('5579J7CFG9PF')
    expect(resolveAffiliateReferralCode()).toBe('5579J7CFG9PF')
  })

  it('expires stale affiliate referral code', () => {
    const now = Date.UTC(2026, 0, 1)
    storeAffiliateReferralCode('AFF123', now)

    expect(loadAffiliateReferralCode(now + 30 * 24 * 60 * 60 * 1000 - 1)).toBe('AFF123')
    expect(loadAffiliateReferralCode(now + 30 * 24 * 60 * 60 * 1000 + 1)).toBe('')
    expect(localStorage.getItem('affiliate_referral_code')).toBeNull()
  })

  it('keeps oauth transient code separate from persistent referral code', () => {
    storeAffiliateReferralCode('PERSISTED')
    storeOAuthAffiliateCode('OAUTH')

    expect(loadAffiliateReferralCode()).toBe('PERSISTED')
    expect(loadOAuthAffiliateCode()).toBe('OAUTH')

    clearOAuthAffiliateCode()
    expect(loadOAuthAffiliateCode()).toBe('')
    expect(loadAffiliateReferralCode()).toBe('PERSISTED')

    clearAffiliateReferralCode()
    expect(loadAffiliateReferralCode()).toBe('')
  })

  it('keeps a sales recruit code in its own store', () => {
    storeSalesReferralCode(' SALES123 ')
    storeAffiliateReferralCode('AFF123')

    expect(loadSalesReferralCode()).toBe('SALES123')
    expect(loadAffiliateReferralCode()).toBe('AFF123')
    expect(localStorage.getItem('sales_referral_code')).toContain('SALES123')
    expect(localStorage.getItem('affiliate_referral_code')).toContain('AFF123')
  })

  it('stores neither code when a registration link carries both', () => {
    storeAffiliateReferralCode('OLD')
    storeSalesReferralCode('OLDER')

    expect(syncExclusiveReferralCodes(['AFF'], ['SALES'])).toEqual({
      aff: '',
      sales: '',
      conflict: true
    })
    expect(loadAffiliateReferralCode()).toBe('')
    expect(loadSalesReferralCode()).toBe('')
  })

  it('stores only the sales code for oauth when the customer code is absent', () => {
    storeOAuthAffiliateCode('STALE')

    expect(prepareOAuthReferralCodes([], ['SALES'])).toEqual({
      aff: '',
      sales: 'SALES',
      conflict: false
    })
    expect(loadOAuthAffiliateCode()).toBe('')
    expect(loadOAuthSalesCode()).toBe('SALES')
    expect(oauthReferralPayload()).toEqual({ sales_code: 'SALES' })
  })

  it('clears both oauth session keys when both codes resolve', () => {
    storeOAuthAffiliateCode('AFF')
    expect(prepareOAuthReferralCodes(['AFF'], ['SALES']).conflict).toBe(true)
    expect(oauthReferralPayload()).toEqual({})
    expect(loadOAuthAffiliateCode()).toBe('')
    expect(loadOAuthSalesCode()).toBe('')
  })

  it('clears sales referral storage with the other affiliate codes', () => {
    storeSalesReferralCode('SALES')
    storeOAuthAffiliateCode('AFF')
    clearAllAffiliateReferralCodes()
    expect(loadSalesReferralCode()).toBe('')
    expect(loadOAuthAffiliateCode()).toBe('')
    expect(loadAffiliateReferralCode()).toBe('')
  })
})
