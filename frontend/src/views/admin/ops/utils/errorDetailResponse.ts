import type { OpsErrorDetail } from '@/api/admin/ops'

const GENERIC_UPSTREAM_MESSAGES = new Set([
  'upstream request failed',
  'upstream request failed after retries',
  'upstream gateway error',
  'upstream service temporarily unavailable'
])

type ParsedGatewayError = {
  type: string
  message: string
}

function parseGatewayErrorBody(raw: string): ParsedGatewayError | null {
  const text = String(raw || '').trim()
  if (!text) return null

  try {
    const parsed = JSON.parse(text) as Record<string, any>
    const err = parsed?.error as Record<string, any> | undefined
    if (!err || typeof err !== 'object') return null

    const type = typeof err.type === 'string' ? err.type.trim() : ''
    const message = typeof err.message === 'string' ? err.message.trim() : ''
    if (!type && !message) return null

    return { type, message }
  } catch {
    return null
  }
}

function isGenericGatewayUpstreamError(raw: string): boolean {
  const parsed = parseGatewayErrorBody(raw)
  if (!parsed) return false
  if (parsed.type !== 'upstream_error') return false
  return GENERIC_UPSTREAM_MESSAGES.has(parsed.message.toLowerCase())
}

export function resolveUpstreamPayload(
  detail: Pick<OpsErrorDetail, 'upstream_error_detail' | 'upstream_errors' | 'upstream_error_message'> | null | undefined
): string {
  if (!detail) return ''

  const candidates = [
    detail.upstream_error_detail,
    detail.upstream_errors,
    detail.upstream_error_message
  ]

  for (const candidate of candidates) {
    const payload = String(candidate || '').trim()
    if (!payload) continue

    // Normalize common "empty but present" JSON placeholders.
    if (payload === '[]' || payload === '{}' || payload.toLowerCase() === 'null') {
      continue
    }

    return payload
  }

  return ''
}

function isEmptyJSONPlaceholder(payload: string): boolean {
  const text = String(payload || '').trim()
  return !text || text === '[]' || text === '{}' || text.toLowerCase() === 'null'
}

export function formatUpstreamOriginal(
  detail: Pick<OpsErrorLogLike, 'provider_error_code' | 'upstream_error_message'> | null | undefined
): string {
  const code = String(detail?.provider_error_code || '').trim()
  const message = String(detail?.upstream_error_message || '').trim()
  if (code && message) return `${code} ${message}`
  return message || code
}

function lastHopUpstreamJSON(raw: string | undefined): string {
  const text = String(raw || '').trim()
  if (isEmptyJSONPlaceholder(text)) return ''
  try {
    const parsed = JSON.parse(text) as unknown
    if (!Array.isArray(parsed) || parsed.length === 0) return ''
    for (let i = parsed.length - 1; i >= 0; i--) {
      const last = parsed[i] as Record<string, unknown> | null
      if (!last || typeof last !== 'object') continue
      const hop = String(last.detail || last.upstream_response_body || '').trim()
      if (!hop || isEmptyJSONPlaceholder(hop) || isGenericGatewayUpstreamError(hop)) continue
      return hop
    }
    return ''
  } catch {
    return ''
  }
}

export function resolveUpstreamJSON(
  detail: Pick<OpsErrorDetail, 'upstream_error_detail' | 'upstream_errors'> | null | undefined
): string {
  if (!detail) return ''
  const rawDetail = String(detail.upstream_error_detail || '').trim()
  if (!isEmptyJSONPlaceholder(rawDetail) && !isGenericGatewayUpstreamError(rawDetail)) {
    return rawDetail
  }
  return lastHopUpstreamJSON(detail.upstream_errors)
}

export function resolveDownstreamJSON(
  detail: Pick<OpsErrorDetail, 'error_body'> | null | undefined
): string {
  return String(detail?.error_body || '').trim()
}

function normalizeComparable(text: string): string {
  return String(text || '').trim().toLowerCase()
}

export function formatOpsListPrimary(
  log: Pick<OpsErrorLogLike, 'provider_error_code' | 'upstream_error_message' | 'message'>
): string {
  return formatUpstreamOriginal(log) || String(log.message || '').trim()
}

export function formatOpsListSecondary(
  log: Pick<OpsErrorLogLike, 'provider_error_code' | 'upstream_error_message' | 'message'>
): string {
  const downstream = String(log.message || '').trim()
  if (!downstream) return ''
  const original = formatUpstreamOriginal(log)
  if (!original) return ''
  if (normalizeComparable(original) === normalizeComparable(downstream)) return ''
  const originalMessage = String(log.upstream_error_message || '').trim()
  if (originalMessage && normalizeComparable(originalMessage) === normalizeComparable(downstream)) {
    return ''
  }
  return downstream
}

export function formatOpsListTitle(
  log: Pick<OpsErrorLogLike, 'provider_error_code' | 'upstream_error_message' | 'message'>
): string {
  const primary = formatOpsListPrimary(log)
  const secondary = formatOpsListSecondary(log)
  if (primary && secondary) return `${primary}\n${secondary}`
  return primary || secondary
}

type OpsErrorLogLike = {
  provider_error_code?: string | null
  upstream_error_message?: string | null
  message?: string | null
}

export function resolvePrimaryResponseBody(
  detail: OpsErrorDetail | null,
  errorType?: 'request' | 'upstream'
): string {
  if (!detail) return ''

  const upstreamPayload = resolveUpstreamPayload(detail)
  const errorBody = String(detail.error_body || '').trim()

  if (errorType === 'upstream') {
    return upstreamPayload || errorBody
  }

  if (!errorBody) {
    return upstreamPayload
  }

  // For request detail modal, keep client-visible body by default.
  // But if that body is a generic gateway wrapper, show upstream payload first.
  if (upstreamPayload && isGenericGatewayUpstreamError(errorBody)) {
    return upstreamPayload
  }

  return errorBody
}
