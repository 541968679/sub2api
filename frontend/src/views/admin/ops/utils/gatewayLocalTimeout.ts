// Keep aligned with backend gatewayLocalTimeoutNeedles. These are limits this
// gateway enforces itself. A provider HTTP timeout is not in this list.
const GATEWAY_LOCAL_TIMEOUT_NEEDLES = [
  'openai_header_wait_timeout',
  'openai_first_useful_frame_timeout',
  'stream data interval timeout',
  'image stream data interval timeout',
  'upstream stream idle for'
]

export interface GatewayTimeoutText {
  message?: string | null
  upstream_error_message?: string | null
  upstream_error_detail?: string | null
  error_body?: string | null
  upstream_errors?: string | null
}

export function isGatewayLocalTimeoutText(...parts: Array<string | null | undefined>): boolean {
  const blob = parts
    .map((part) => String(part || '').trim())
    .filter(Boolean)
    .join('\n')
    .toLowerCase()
  if (!blob) return false
  return GATEWAY_LOCAL_TIMEOUT_NEEDLES.some((needle) => blob.includes(needle))
}

export function isGatewayLocalTimeoutLog(log: GatewayTimeoutText | null | undefined): boolean {
  if (!log) return false
  return isGatewayLocalTimeoutText(
    log.message,
    log.upstream_error_message,
    log.upstream_error_detail,
    log.error_body,
    log.upstream_errors
  )
}
