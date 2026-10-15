const OAUTH_AFFILIATE_CODE_KEY = 'oauth_aff_code'
const OAUTH_SALES_CODE_KEY = 'oauth_sales_code'
const AFFILIATE_REFERRAL_CODE_KEY = 'affiliate_referral_code'
const SALES_REFERRAL_CODE_KEY = 'sales_referral_code'
const AFFILIATE_REFERRAL_TTL_MS = 30 * 24 * 60 * 60 * 1000

export interface ExclusiveReferral {
  aff: string
  sales: string
  conflict: boolean
}

interface StoredAffiliateReferralCode {
  code: string
  expiresAt: number
}

export function normalizeOAuthAffiliateCode(value?: unknown): string {
  const raw = Array.isArray(value) ? value[0] : value
  return typeof raw === 'string' ? raw.trim() : ''
}

export function pickOAuthAffiliateCode(...values: unknown[]): string {
  for (const value of values) {
    const code = normalizeOAuthAffiliateCode(value)
    if (code) {
      return code
    }
  }
  return ''
}

export function storeAffiliateReferralCode(value?: unknown, now = Date.now()): void {
  if (typeof window === 'undefined') {
    return
  }
  const code = normalizeOAuthAffiliateCode(value)
  if (!code) {
    return
  }
  try {
    const payload: StoredAffiliateReferralCode = {
      code,
      expiresAt: now + AFFILIATE_REFERRAL_TTL_MS
    }
    window.localStorage.setItem(AFFILIATE_REFERRAL_CODE_KEY, JSON.stringify(payload))
  } catch {
    // 忽略浏览器存储异常。
  }
}

export function loadAffiliateReferralCode(now = Date.now()): string {
  if (typeof window === 'undefined') {
    return ''
  }
  try {
    const raw = window.localStorage.getItem(AFFILIATE_REFERRAL_CODE_KEY)
    if (!raw) {
      return ''
    }
    const parsed = JSON.parse(raw) as Partial<StoredAffiliateReferralCode>
    const code = normalizeOAuthAffiliateCode(parsed.code)
    const expiresAt = Number(parsed.expiresAt) || 0
    if (!code || expiresAt <= now) {
      clearAffiliateReferralCode()
      return ''
    }
    return code
  } catch {
    clearAffiliateReferralCode()
    return ''
  }
}

export function clearAffiliateReferralCode(): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.localStorage.removeItem(AFFILIATE_REFERRAL_CODE_KEY)
  } catch {
    // 忽略浏览器存储异常。
  }
}

export function resolveAffiliateReferralCode(...values: unknown[]): string {
  const code = pickOAuthAffiliateCode(...values)
  if (code) {
    storeAffiliateReferralCode(code)
    return code
  }
  return loadAffiliateReferralCode()
}

export function storeOAuthAffiliateCode(value?: unknown): void {
  if (typeof window === 'undefined') {
    return
  }
  const code = normalizeOAuthAffiliateCode(value)
  try {
    if (code) {
      window.sessionStorage.setItem(OAUTH_AFFILIATE_CODE_KEY, code)
    } else {
      window.sessionStorage.removeItem(OAUTH_AFFILIATE_CODE_KEY)
    }
  } catch {
    // 忽略浏览器存储异常。
  }
}

export function loadOAuthAffiliateCode(): string {
  if (typeof window === 'undefined') {
    return ''
  }
  try {
    return normalizeOAuthAffiliateCode(window.sessionStorage.getItem(OAUTH_AFFILIATE_CODE_KEY))
  } catch {
    return ''
  }
}

export function clearOAuthAffiliateCode(): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.sessionStorage.removeItem(OAUTH_AFFILIATE_CODE_KEY)
  } catch {
    // 忽略浏览器存储异常。
  }
}

export function storeSalesReferralCode(value?: unknown, now = Date.now()): void {
  if (typeof window === 'undefined') {
    return
  }
  const code = normalizeOAuthAffiliateCode(value)
  if (!code) {
    return
  }
  try {
    const payload: StoredAffiliateReferralCode = {
      code,
      expiresAt: now + AFFILIATE_REFERRAL_TTL_MS
    }
    window.localStorage.setItem(SALES_REFERRAL_CODE_KEY, JSON.stringify(payload))
  } catch {
    // 忽略浏览器存储异常。
  }
}

export function loadSalesReferralCode(now = Date.now()): string {
  if (typeof window === 'undefined') {
    return ''
  }
  try {
    const raw = window.localStorage.getItem(SALES_REFERRAL_CODE_KEY)
    if (!raw) {
      return ''
    }
    const parsed = JSON.parse(raw) as Partial<StoredAffiliateReferralCode>
    const code = normalizeOAuthAffiliateCode(parsed.code)
    const expiresAt = Number(parsed.expiresAt) || 0
    if (!code || expiresAt <= now) {
      clearSalesReferralCode()
      return ''
    }
    return code
  } catch {
    clearSalesReferralCode()
    return ''
  }
}

export function clearSalesReferralCode(): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.localStorage.removeItem(SALES_REFERRAL_CODE_KEY)
  } catch {
    // 忽略浏览器存储异常。
  }
}

export function storeOAuthSalesCode(value?: unknown): void {
  if (typeof window === 'undefined') {
    return
  }
  const code = normalizeOAuthAffiliateCode(value)
  try {
    if (code) {
      window.sessionStorage.setItem(OAUTH_SALES_CODE_KEY, code)
    } else {
      window.sessionStorage.removeItem(OAUTH_SALES_CODE_KEY)
    }
  } catch {
    // 忽略浏览器存储异常。
  }
}

export function loadOAuthSalesCode(): string {
  if (typeof window === 'undefined') {
    return ''
  }
  try {
    return normalizeOAuthAffiliateCode(window.sessionStorage.getItem(OAUTH_SALES_CODE_KEY))
  } catch {
    return ''
  }
}

export function clearOAuthSalesCode(): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.sessionStorage.removeItem(OAUTH_SALES_CODE_KEY)
  } catch {
    // 忽略浏览器存储异常。
  }
}

// One registration carries one kind of code. Both present means store and send neither.
export function syncExclusiveReferralCodes(affValues: unknown[], salesValues: unknown[]): ExclusiveReferral {
  const explicitAff = pickOAuthAffiliateCode(...affValues)
  const explicitSales = pickOAuthAffiliateCode(...salesValues)
  if (explicitAff && explicitSales) {
    clearAffiliateReferralCode()
    clearSalesReferralCode()
    return { aff: '', sales: '', conflict: true }
  }
  if (explicitAff) {
    storeAffiliateReferralCode(explicitAff)
    clearSalesReferralCode()
    return { aff: explicitAff, sales: '', conflict: false }
  }
  if (explicitSales) {
    storeSalesReferralCode(explicitSales)
    clearAffiliateReferralCode()
    return { aff: '', sales: explicitSales, conflict: false }
  }
  const aff = loadAffiliateReferralCode()
  const sales = loadSalesReferralCode()
  if (aff && sales) {
    clearAffiliateReferralCode()
    clearSalesReferralCode()
    return { aff: '', sales: '', conflict: true }
  }
  return { aff, sales, conflict: false }
}

export function prepareOAuthReferralCodes(affValues: unknown[], salesValues: unknown[]): ExclusiveReferral {
  const resolved = syncExclusiveReferralCodes(affValues, salesValues)
  if (resolved.conflict) {
    clearOAuthAffiliateCode()
    clearOAuthSalesCode()
    return resolved
  }
  storeOAuthAffiliateCode(resolved.aff)
  storeOAuthSalesCode(resolved.sales)
  return resolved
}

export function clearAllAffiliateReferralCodes(): void {
  clearOAuthAffiliateCode()
  clearOAuthSalesCode()
  clearAffiliateReferralCode()
  clearSalesReferralCode()
}

export function oauthAffiliatePayload(value?: unknown): { aff_code?: string } {
  const code = normalizeOAuthAffiliateCode(value)
  return code ? { aff_code: code } : {}
}

export function exclusiveReferralFields(
  affiliateCode?: unknown,
  salesCode?: unknown
): { aff_code?: string; sales_code?: string } {
  const aff = normalizeOAuthAffiliateCode(affiliateCode)
  const sales = normalizeOAuthAffiliateCode(salesCode)
  if (aff && sales) {
    return {}
  }
  if (sales) {
    return { sales_code: sales }
  }
  if (aff) {
    return { aff_code: aff }
  }
  return {}
}

export function oauthReferralPayload(): { aff_code?: string; sales_code?: string } {
  return exclusiveReferralFields(loadOAuthAffiliateCode(), loadOAuthSalesCode())
}
