const AUTH_STORAGE_KEYS = ['auth_token', 'refresh_token', 'auth_user', 'token_expires_at'] as const

export const CURRENT_LEGAL_CONSENT_VERSION = '2026-06-11-v1'
const CONSENT_STORAGE_PREFIX = 'legal_consent:user:'
const FORCE_LOGOUT_STORAGE_KEY = 'legal_consent:force_logout_version'

export interface LegalConsentPayload {
  typedConfirmation: string
  dwellSeconds: number
  scrolledToBottom: boolean
  regionAttestation: boolean
  source: 'register' | 'login' | 'email_verify'
}

interface StoredLegalConsent extends LegalConsentPayload {
  version: string
  acceptedAt: string
}

function userConsentStorageKey(userID: number | string): string {
  return `${CONSENT_STORAGE_PREFIX}${String(userID)}`
}

export function hasAcceptedCurrentLegalConsent(userID: number | string | null | undefined): boolean {
  if (userID === null || userID === undefined || userID === '') {
    return false
  }

  try {
    const raw = localStorage.getItem(userConsentStorageKey(userID))
    if (!raw) {
      return false
    }
    const parsed = JSON.parse(raw) as Partial<StoredLegalConsent>
    return (
      parsed.version === CURRENT_LEGAL_CONSENT_VERSION &&
      parsed.scrolledToBottom === true &&
      parsed.regionAttestation === true &&
      typeof parsed.typedConfirmation === 'string' &&
      parsed.typedConfirmation.trim().length > 0
    )
  } catch {
    localStorage.removeItem(userConsentStorageKey(userID))
    return false
  }
}

export function markLegalConsentAccepted(
  userID: number | string | null | undefined,
  payload: LegalConsentPayload
): void {
  if (userID === null || userID === undefined || userID === '') {
    return
  }

  const stored: StoredLegalConsent = {
    ...payload,
    version: CURRENT_LEGAL_CONSENT_VERSION,
    acceptedAt: new Date().toISOString()
  }

  localStorage.setItem(userConsentStorageKey(userID), JSON.stringify(stored))
}

export function getPendingRegisterLegalConsent(): LegalConsentPayload | null {
  try {
    const raw = sessionStorage.getItem('register_legal_consent')
    if (!raw) {
      return null
    }
    return JSON.parse(raw) as LegalConsentPayload
  } catch {
    sessionStorage.removeItem('register_legal_consent')
    return null
  }
}

export function storePendingRegisterLegalConsent(payload: LegalConsentPayload): void {
  sessionStorage.setItem('register_legal_consent', JSON.stringify(payload))
}

export function clearPendingRegisterLegalConsent(): void {
  sessionStorage.removeItem('register_legal_consent')
}

export function clearStaleAuthForLegalConsent(): boolean {
  if (localStorage.getItem(FORCE_LOGOUT_STORAGE_KEY) === CURRENT_LEGAL_CONSENT_VERSION) {
    return false
  }

  const hadAuthState = AUTH_STORAGE_KEYS.some((key) => localStorage.getItem(key) !== null)
  for (const key of AUTH_STORAGE_KEYS) {
    localStorage.removeItem(key)
  }
  localStorage.setItem(FORCE_LOGOUT_STORAGE_KEY, CURRENT_LEGAL_CONSENT_VERSION)

  return hadAuthState
}
