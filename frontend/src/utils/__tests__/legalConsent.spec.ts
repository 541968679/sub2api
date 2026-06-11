import { beforeEach, describe, expect, it } from 'vitest'
import {
  CURRENT_LEGAL_CONSENT_VERSION,
  clearStaleAuthForLegalConsent,
  hasAcceptedCurrentLegalConsent,
  markLegalConsentAccepted
} from '@/utils/legalConsent'

describe('legal consent persistence', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('stores acceptance by user and current legal version only', () => {
    expect(hasAcceptedCurrentLegalConsent(1)).toBe(false)

    markLegalConsentAccepted(1, {
      typedConfirmation: '我已阅读并同意全部条款',
      dwellSeconds: 32,
      scrolledToBottom: true,
      regionAttestation: true,
      source: 'register'
    })

    expect(hasAcceptedCurrentLegalConsent(1)).toBe(true)
    expect(hasAcceptedCurrentLegalConsent(2)).toBe(false)
  })

  it('does not accept stale legal versions', () => {
    localStorage.setItem(
      'legal_consent:user:9',
      JSON.stringify({
        version: 'stale-version',
        acceptedAt: new Date().toISOString()
      })
    )

    expect(CURRENT_LEGAL_CONSENT_VERSION).not.toBe('stale-version')
    expect(hasAcceptedCurrentLegalConsent(9)).toBe(false)
  })

  it('clears existing auth tokens once for a new legal force-logout version', () => {
    localStorage.setItem('auth_token', 'old-token')
    localStorage.setItem('refresh_token', 'old-refresh')
    localStorage.setItem('auth_user', JSON.stringify({ id: 1 }))
    localStorage.setItem('token_expires_at', '9999999999999')

    expect(clearStaleAuthForLegalConsent()).toBe(true)
    expect(localStorage.getItem('auth_token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
    expect(localStorage.getItem('auth_user')).toBeNull()
    expect(localStorage.getItem('token_expires_at')).toBeNull()
    expect(localStorage.getItem('legal_consent:force_logout_version')).toBe(CURRENT_LEGAL_CONSENT_VERSION)

    localStorage.setItem('auth_token', 'new-token')
    expect(clearStaleAuthForLegalConsent()).toBe(false)
    expect(localStorage.getItem('auth_token')).toBe('new-token')
  })
})
