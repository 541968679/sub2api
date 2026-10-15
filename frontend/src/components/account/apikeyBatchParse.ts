import { domesticPlatformDefaultBaseURL } from '@/utils/domesticPlatformChoice'

export const TOKENRHYTHM_ORIGIN = 'https://tokenrhythm.studio'
export const API_KEY_IMPORT_MAX = 500

export type APIKeyImportReason = 'missing_key' | 'duplicate' | 'unrecognized'

export interface ParsedAPIKey {
  apiKey: string
  sessionToken?: string
  refreshToken?: string
  line: number
}

export interface APIKeyImportError {
  line: number
  reason: APIKeyImportReason
}

export interface APIKeyImportParse {
  entries: ParsedAPIKey[]
  errors: APIKeyImportError[]
}

const FIELD_SPLIT = '----'

function isIgnorablePart(part: string): boolean {
  if (/^\+?\d{6,15}$/.test(part)) return true
  if (/^[-+]?\d+(?:\.\d+)?$/.test(part)) return true
  return /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/.test(part)
}

function splitFields(line: string): string[] {
  const normalized = line.includes(FIELD_SPLIT) ? line : line.replace(/\|/g, FIELD_SPLIT)
  return normalized
    .split(FIELD_SPLIT)
    .map((part) => part.trim())
    .filter(Boolean)
}

function parseStructuredLine(line: string, lineNo: number): { entry?: ParsedAPIKey; error?: APIKeyImportError } {
  const parts = splitFields(line)
  if (parts.length === 0) {
    return { error: { line: lineNo, reason: 'unrecognized' } }
  }
  if (parts.length === 1 && !line.includes(FIELD_SPLIT) && !line.includes('|')) {
    return { entry: { apiKey: parts[0], line: lineNo } }
  }

  let apiKey = ''
  let sessionToken = ''
  let refreshToken = ''
  const unknown: string[] = []
  for (const part of parts) {
    if (part.startsWith('sess_')) {
      if (!sessionToken) sessionToken = part
      continue
    }
    if (part.startsWith('rf_')) {
      if (!refreshToken) refreshToken = part
      continue
    }
    if (part.startsWith('sk_') || part.startsWith('sk-')) {
      if (apiKey) return { error: { line: lineNo, reason: 'unrecognized' } }
      apiKey = part
      continue
    }
    if (isIgnorablePart(part)) continue
    unknown.push(part)
  }
  if (!apiKey && unknown.length === 1) {
    apiKey = unknown[0]
  } else if (!apiKey) {
    return { error: { line: lineNo, reason: unknown.length > 1 ? 'unrecognized' : 'missing_key' } }
  }
  const entry: ParsedAPIKey = { apiKey, line: lineNo }
  if (sessionToken) entry.sessionToken = sessionToken
  if (refreshToken) entry.refreshToken = refreshToken
  return { entry }
}

export function parseAPIKeyImport(raw: string): APIKeyImportParse {
  const entries: ParsedAPIKey[] = []
  const errors: APIKeyImportError[] = []
  const seen = new Set<string>()
  const lines = raw.replace(/\r\n/g, '\n').replace(/\r/g, '\n').split('\n')
  lines.forEach((line, index) => {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) return
    const parsed = parseStructuredLine(trimmed, index + 1)
    if (parsed.error) {
      errors.push(parsed.error)
      return
    }
    const entry = parsed.entry
    if (!entry) return
    if (seen.has(entry.apiKey)) {
      errors.push({ line: entry.line, reason: 'duplicate' })
      return
    }
    seen.add(entry.apiKey)
    entries.push(entry)
  })
  return { entries, errors }
}

export function apiKeyImportTail(apiKey: string, index: number): string {
  const tail = apiKey.replace(/[^A-Za-z0-9]/g, '').slice(-4)
  return tail || String(index + 1)
}

export function assignAPIKeyImportNames(prefix: string, apiKeys: string[]): string[] {
  const base = prefix.trim()
  const used = new Set<string>()
  return apiKeys.map((apiKey, index) => {
    let name = `${base}-${apiKeyImportTail(apiKey, index)}`
    if (used.has(name)) name = `${name}-${index + 1}`
    used.add(name)
    return name
  })
}

export function platformDefaultAPIKeyBaseURL(platform: string): string {
  const domestic = domesticPlatformDefaultBaseURL(platform)
  if (domestic) return domestic
  if (platform === 'openai') return 'https://api.openai.com'
  if (platform === 'gemini') return 'https://generativelanguage.googleapis.com'
  if (platform === 'grok') return 'https://api.x.ai/v1'
  return 'https://api.anthropic.com'
}

export function resolveAPIKeyImportBaseURL(typed: string, platformDefault: string, hasSession: boolean): string {
  const value = typed.trim()
  const fallback = platformDefault.trim()
  if (hasSession && (value === '' || value === fallback)) {
    return TOKENRHYTHM_ORIGIN
  }
  return value || fallback
}

export function applyTokenRhythmCredentials(
  credentials: Record<string, unknown>,
  entry: Pick<ParsedAPIKey, 'sessionToken' | 'refreshToken'>
): void {
  if (entry.sessionToken) credentials.tokenrhythm_session_token = entry.sessionToken
  else delete credentials.tokenrhythm_session_token
  if (entry.refreshToken) credentials.tokenrhythm_refresh_token = entry.refreshToken
  else delete credentials.tokenrhythm_refresh_token
}
