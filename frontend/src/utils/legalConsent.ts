import type { LegalConsentSettings } from '@/types'

const AUTH_STORAGE_KEYS = ['auth_token', 'refresh_token', 'auth_user', 'token_expires_at'] as const

export const CURRENT_LEGAL_CONSENT_VERSION = '2026-06-11-internal-research-v2'
export const CURRENT_LEGAL_CONFIRMATION_PHRASE =
  '我确认本人为授权内部测试人员，已阅读并同意上述使用条款与免责声明，知悉本平台非商业服务且不提供在线充值，如有任何风险或问题由本人自行承担'
export const DEFAULT_LEGAL_CONSENT_CONTENT = `⚠️ 重要声明
本平台仅作为内部技术研究、模型接口测试、系统联调及数据验证用途。

平台不面向公众开放注册、充值、购买、代充、转售或任何商业化使用。

非授权人员请立即停止访问。

🔬 内部科研用途
本系统用于内部大模型接口兼容性测试、模型能力验证、请求日志分析、计费规则模拟及技术架构研究。

🔐 权限使用说明
平台账号仅限授权人员本人使用，不得转借、共享、出租、出售或提供给任何第三方使用。

📌 使用协议
一、使用范围：
本平台仅限内部授权人员用于科研测试、技术验证及系统维护，不提供任何对外服务。

二、禁止行为：
禁止将本平台用于公开运营、商业销售、接口转售、批量注册、违法违规内容生成、恶意请求、压力攻击、数据抓取或任何违反法律法规的用途。

三、账号安全：
用户应妥善保管账号、密码及令牌 Key。因个人泄露、转借、共享导致的风险和损失，由使用者自行承担。

四、风控处理：
如发现异常访问、接口滥用、违规调用、恶意测试、传播平台信息等行为，平台有权立即限制、暂停或关闭相关权限。

🚫 本平台当前不提供在线充值、不开放公众注册、不承接外部客户、不进行公开商业运营。

继续访问或使用本平台，即表示您已阅读并同意以上公告及相关协议内容。`

export const DEFAULT_LEGAL_CONSENT_SETTINGS: LegalConsentSettings = {
  enabled: true,
  version: CURRENT_LEGAL_CONSENT_VERSION,
  content: DEFAULT_LEGAL_CONSENT_CONTENT,
  confirmation_phrase: CURRENT_LEGAL_CONFIRMATION_PHRASE,
  min_read_seconds: 20
}

const CONSENT_STORAGE_PREFIX = 'legal_consent:user:'
const FORCE_LOGOUT_STORAGE_KEY = 'legal_consent:force_logout_version'
const PENDING_REGISTER_LEGAL_CONSENT_KEY = 'register_legal_consent'

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

interface PendingLegalConsent extends LegalConsentPayload {
  version: string
}

type LegalConsentSettingsInput = Partial<LegalConsentSettings> | null | undefined

function userConsentStorageKey(userID: number | string): string {
  return `${CONSENT_STORAGE_PREFIX}${String(userID)}`
}

export function resolveLegalConsentSettings(settings?: LegalConsentSettingsInput): LegalConsentSettings {
  return {
    enabled: settings?.enabled ?? DEFAULT_LEGAL_CONSENT_SETTINGS.enabled,
    version: normalizeNonEmpty(settings?.version, DEFAULT_LEGAL_CONSENT_SETTINGS.version),
    content: normalizeNonEmpty(settings?.content, DEFAULT_LEGAL_CONSENT_SETTINGS.content),
    confirmation_phrase: normalizeNonEmpty(
      settings?.confirmation_phrase,
      DEFAULT_LEGAL_CONSENT_SETTINGS.confirmation_phrase
    ),
    min_read_seconds: normalizeMinReadSeconds(settings?.min_read_seconds)
  }
}

function normalizeNonEmpty(value: unknown, fallback: string): string {
  return typeof value === 'string' && value.trim() ? value.trim() : fallback
}

function normalizeMinReadSeconds(value: unknown): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed)) {
    return DEFAULT_LEGAL_CONSENT_SETTINGS.min_read_seconds
  }
  return Math.max(0, Math.min(300, Math.trunc(parsed)))
}

function isCurrentLegalConsentPayload(
  payload: Partial<LegalConsentPayload> | null | undefined,
  settings?: LegalConsentSettingsInput
): boolean {
  const resolved = resolveLegalConsentSettings(settings)
  if (!resolved.enabled) {
    return true
  }
  return (
    payload?.scrolledToBottom === true &&
    payload.authorizedUseAttestation === true &&
    typeof payload.typedConfirmation === 'string' &&
    payload.typedConfirmation.trim() === resolved.confirmation_phrase
  )
}

export function hasAcceptedCurrentLegalConsent(
  userID: number | string | null | undefined,
  settings?: LegalConsentSettingsInput
): boolean {
  const resolved = resolveLegalConsentSettings(settings)
  if (!resolved.enabled) {
    return true
  }
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
      parsed.version === resolved.version &&
      isCurrentLegalConsentPayload(parsed, resolved)
    )
  } catch {
    localStorage.removeItem(userConsentStorageKey(userID))
    return false
  }
}

export function markLegalConsentAccepted(
  userID: number | string | null | undefined,
  payload: LegalConsentPayload,
  settings?: LegalConsentSettingsInput
): void {
  const resolved = resolveLegalConsentSettings(settings)
  if (!resolved.enabled || userID === null || userID === undefined || userID === '') {
    return
  }

  const stored: StoredLegalConsent = {
    ...payload,
    version: resolved.version,
    acceptedAt: new Date().toISOString()
  }

  localStorage.setItem(userConsentStorageKey(userID), JSON.stringify(stored))
  localStorage.setItem(FORCE_LOGOUT_STORAGE_KEY, resolved.version)
}

export function getPendingRegisterLegalConsent(settings?: LegalConsentSettingsInput): LegalConsentPayload | null {
  const resolved = resolveLegalConsentSettings(settings)
  if (!resolved.enabled) {
    return null
  }

  try {
    const raw = sessionStorage.getItem(PENDING_REGISTER_LEGAL_CONSENT_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw) as Partial<PendingLegalConsent>
    if (parsed.version !== resolved.version || !isCurrentLegalConsentPayload(parsed, resolved)) {
      sessionStorage.removeItem(PENDING_REGISTER_LEGAL_CONSENT_KEY)
      return null
    }
    return parsed as LegalConsentPayload
  } catch {
    sessionStorage.removeItem(PENDING_REGISTER_LEGAL_CONSENT_KEY)
    return null
  }
}

export function storePendingRegisterLegalConsent(
  payload: LegalConsentPayload,
  settings?: LegalConsentSettingsInput
): void {
  const resolved = resolveLegalConsentSettings(settings)
  if (!resolved.enabled) {
    return
  }
  const pending: PendingLegalConsent = {
    ...payload,
    version: resolved.version
  }
  sessionStorage.setItem(PENDING_REGISTER_LEGAL_CONSENT_KEY, JSON.stringify(pending))
}

export function clearPendingRegisterLegalConsent(): void {
  sessionStorage.removeItem(PENDING_REGISTER_LEGAL_CONSENT_KEY)
}

export function clearStaleAuthForLegalConsent(settings?: LegalConsentSettingsInput): boolean {
  const resolved = resolveLegalConsentSettings(settings)
  if (!resolved.enabled) {
    return false
  }
  if (localStorage.getItem(FORCE_LOGOUT_STORAGE_KEY) === resolved.version) {
    return false
  }

  try {
    const rawUser = localStorage.getItem('auth_user')
    const parsedUser = rawUser ? JSON.parse(rawUser) as { id?: number | string | null } : null
    if (hasAcceptedCurrentLegalConsent(parsedUser?.id, resolved)) {
      localStorage.setItem(FORCE_LOGOUT_STORAGE_KEY, resolved.version)
      return false
    }
  } catch {
    // Malformed auth_user is cleared by the auth store during bootstrap.
  }

  const hadAuthState = AUTH_STORAGE_KEYS.some((key) => localStorage.getItem(key) !== null)
  for (const key of AUTH_STORAGE_KEYS) {
    localStorage.removeItem(key)
  }
  localStorage.setItem(FORCE_LOGOUT_STORAGE_KEY, resolved.version)

  return hadAuthState
}
