import { beforeEach, describe, expect, it } from 'vitest'
import {
  CURRENT_LEGAL_CONFIRMATION_PHRASE,
  CURRENT_LEGAL_CONSENT_VERSION,
  clearStaleAuthForLegalConsent,
  DEFAULT_LEGAL_CONSENT_SETTINGS,
  getPendingRegisterLegalConsent,
  hasAcceptedCurrentLegalConsent,
  markLegalConsentAccepted,
  resolveLegalConsentSettings,
  storePendingRegisterLegalConsent
} from '@/utils/legalConsent'

describe('legal consent persistence', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  it('uses the internal research terms version', () => {
    expect(CURRENT_LEGAL_CONSENT_VERSION).toBe('2026-06-11-internal-research-v2')
  })

  it('invalidates acceptance when a configured legal version changes', () => {
    const v3 = {
      ...DEFAULT_LEGAL_CONSENT_SETTINGS,
      version: 'legal-v3',
      confirmation_phrase: 'I agree to legal-v3'
    }
    markLegalConsentAccepted(1, {
      typedConfirmation: 'I agree to legal-v3',
      dwellSeconds: 20,
      scrolledToBottom: true,
      authorizedUseAttestation: true,
      source: 'login'
    }, v3)

    expect(hasAcceptedCurrentLegalConsent(1, v3)).toBe(true)
    expect(hasAcceptedCurrentLegalConsent(1, { ...v3, version: 'legal-v4' })).toBe(false)
  })

  it('records the accepted version as the current force-logout version', () => {
    const v3 = {
      ...DEFAULT_LEGAL_CONSENT_SETTINGS,
      version: 'legal-v3',
      confirmation_phrase: 'I agree to legal-v3'
    }

    markLegalConsentAccepted(1, {
      typedConfirmation: 'I agree to legal-v3',
      dwellSeconds: 20,
      scrolledToBottom: true,
      authorizedUseAttestation: true,
      source: 'login'
    }, v3)

    expect(localStorage.getItem('legal_consent:force_logout_version')).toBe('legal-v3')
  })

  it('treats disabled legal consent as already accepted', () => {
    const disabled = resolveLegalConsentSettings({ enabled: false })
    expect(hasAcceptedCurrentLegalConsent(1, disabled)).toBe(true)
    expect(clearStaleAuthForLegalConsent(disabled)).toBe(false)
  })

  it('stores acceptance by user and current legal version only', () => {
    expect(hasAcceptedCurrentLegalConsent(1)).toBe(false)

    markLegalConsentAccepted(1, {
      typedConfirmation: CURRENT_LEGAL_CONFIRMATION_PHRASE,
      dwellSeconds: 32,
      scrolledToBottom: true,
      authorizedUseAttestation: true,
      source: 'register'
    })

    expect(hasAcceptedCurrentLegalConsent(1)).toBe(true)
    expect(hasAcceptedCurrentLegalConsent(2)).toBe(false)
  })

  it('does not accept stale legal versions', () => {
    localStorage.setItem(
      'legal_consent:user:9',
      JSON.stringify({
        version: CURRENT_LEGAL_CONSENT_VERSION,
        acceptedAt: new Date().toISOString(),
        typedConfirmation: CURRENT_LEGAL_CONFIRMATION_PHRASE,
        scrolledToBottom: true,
        regionAttestation: true
      })
    )

    expect(hasAcceptedCurrentLegalConsent(9)).toBe(false)
  })

  it('only returns pending register consent for the current internal terms payload', () => {
    sessionStorage.setItem(
      'register_legal_consent',
      JSON.stringify({
        typedConfirmation: '我已同意上述条款，如有任何风险或问题自行承担',
        dwellSeconds: 20,
        scrolledToBottom: true,
        regionAttestation: true,
        source: 'register'
      })
    )

    expect(getPendingRegisterLegalConsent()).toBeNull()
    expect(sessionStorage.getItem('register_legal_consent')).toBeNull()

    storePendingRegisterLegalConsent({
      typedConfirmation: CURRENT_LEGAL_CONFIRMATION_PHRASE,
      dwellSeconds: 20,
      scrolledToBottom: true,
      authorizedUseAttestation: true,
      source: 'register'
    })

    expect(getPendingRegisterLegalConsent()).toMatchObject({
      typedConfirmation: CURRENT_LEGAL_CONFIRMATION_PHRASE,
      authorizedUseAttestation: true,
      source: 'register'
    })
  })

  it('invalidates pending register consent when configured version changes', () => {
    const v3 = {
      ...DEFAULT_LEGAL_CONSENT_SETTINGS,
      version: 'legal-v3',
      confirmation_phrase: 'I agree to legal-v3'
    }

    storePendingRegisterLegalConsent({
      typedConfirmation: 'I agree to legal-v3',
      dwellSeconds: 20,
      scrolledToBottom: true,
      authorizedUseAttestation: true,
      source: 'register'
    }, v3)

    expect(getPendingRegisterLegalConsent(v3)).toMatchObject({
      typedConfirmation: 'I agree to legal-v3',
      authorizedUseAttestation: true,
      source: 'register'
    })
    expect(getPendingRegisterLegalConsent({ ...v3, version: 'legal-v4' })).toBeNull()
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
