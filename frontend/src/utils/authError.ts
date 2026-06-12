interface APIErrorLike {
  reason?: string
  message?: string
  response?: {
    data?: {
      reason?: string
      detail?: string
      message?: string
    }
  }
}

function extractErrorMessage(error: unknown): string {
  const err = (error || {}) as APIErrorLike
  return err.response?.data?.detail || err.response?.data?.message || err.message || ''
}

export function buildAuthErrorMessage(
  error: unknown,
  options: {
    fallback: string
    pendingApproval?: string
  }
): string {
  const { fallback } = options
  const err = (error || {}) as APIErrorLike
  const reason = err.reason || err.response?.data?.reason
  if (reason === 'USER_PENDING_APPROVAL') {
    return options.pendingApproval || fallback
  }
  const message = extractErrorMessage(error)
  return message || fallback
}
