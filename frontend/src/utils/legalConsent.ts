const AUTH_STORAGE_KEYS = ['auth_token', 'refresh_token', 'auth_user', 'token_expires_at'] as const

export const CURRENT_LEGAL_CONSENT_VERSION = '2026-06-11-internal-research-v2'
export const CURRENT_LEGAL_CONFIRMATION_PHRASE =
  '我确认本人为授权内部测试人员，已阅读并同意上述使用条款与免责声明，知悉本平台非商业服务且不提供在线充值，如有任何风险或问题由本人自行承担'
const CONSENT_STORAGE_PREFIX = 'legal_consent:user:'
const FORCE_LOGOUT_STORAGE_KEY = 'legal_consent:force_logout_version'

export interface LegalConsentPayload {
  typedConfirmation: string
  dwellSeconds: number
  scrolledToBottom: boolean
  authorizedUseAttestation: boolean
  source: 'register' | 'login' | 'email_verify'
}

interface StoredLegalConsent extends LegalConsentPayload {
  version: string
  acceptedAt: string
}

function userConsentStorageKey(userID: number | string): string {
  return `${CONSENT_STORAGE_PREFIX}${String(userID)}`
}

function isCurrentLegalConsentPayload(payload: Partial<LegalConsentPayload> | null | undefined): boolean {
  return (
    payload?.scrolledToBottom === true &&
    payload.authorizedUseAttestation === true &&
    typeof payload.typedConfirmation === 'string' &&
    payload.typedConfirmation.trim() === CURRENT_LEGAL_CONFIRMATION_PHRASE
  )
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
      isCurrentLegalConsentPayload(parsed)
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
    const parsed = JSON.parse(raw) as Partial<LegalConsentPayload>
    if (!isCurrentLegalConsentPayload(parsed)) {
      sessionStorage.removeItem('register_legal_consent')
      return null
    }
    return parsed as LegalConsentPayload
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
