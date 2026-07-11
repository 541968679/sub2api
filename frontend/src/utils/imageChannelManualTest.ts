import type {
  ImageChannelManualExecutionMode,
  ImageChannelManualRunResponse,
  ImageChannelManualTestParams,
} from '@/api/admin/imageChannelMonitor'

const manualDeliveryBufferSeconds = 15
const manualObservationBufferSeconds = 30
const manualArtifactRetentionMs = 30 * 60 * 1000

export function manualRunObservationMaxAttempts(
  executionMode: ImageChannelManualExecutionMode,
  timeoutSeconds: number
): number | undefined {
  if (executionMode !== 'direct_probe') return undefined
  const parsed = Number(timeoutSeconds)
  const deliverySeconds = Number.isFinite(parsed)
    ? Math.min(600, Math.max(30, Math.trunc(parsed)))
    : 300
  return deliverySeconds + manualDeliveryBufferSeconds + manualObservationBufferSeconds
}

export function manualArtifactRecoveryExpiresAt(completedAt: string): string {
  const completedAtMs = Date.parse(completedAt)
  if (!Number.isFinite(completedAtMs)) return ''
  return new Date(completedAtMs + manualArtifactRetentionMs).toISOString()
}

export function revokeManualObjectURLsForRun(
  objectURLs: Map<string, string>,
  runID: string,
  revoke: (objectURL: string) => void = URL.revokeObjectURL
) {
  const persistedPrefix = `${runID}:`
  for (const [key, objectURL] of objectURLs) {
    if (key !== runID && !key.startsWith(persistedPrefix)) continue
    revoke(objectURL)
    objectURLs.delete(key)
  }
}

export interface ManualRunInputImage {
  data: string
  blob?: Blob
  type: string
  name: string
}

export interface BuildManualRunRequestsOptions {
  targetIds: number[]
  concurrency: number
  batchId: string
  basePayload: ImageChannelManualTestParams
  inputImages?: ManualRunInputImage[]
}

export interface ManualRunRequest {
  recordId: string
  targetId: number
  batchIndex: number
  payload: ImageChannelManualTestParams
  inputImage?: ManualRunInputImage
}

export function buildManualRunRequests({
  targetIds,
  concurrency,
  batchId,
  basePayload,
  inputImages = [],
}: BuildManualRunRequestsOptions): ManualRunRequest[] {
  if (!Number.isSafeInteger(concurrency) || concurrency < 1) {
    throw new RangeError('Manual test concurrency must be a positive integer')
  }

  const totalRuns = targetIds.length * concurrency
  if (basePayload.mode === 'edit' && inputImages.length === 0) {
    throw new Error('Manual edit tests require at least one input image')
  }

  const requests: ManualRunRequest[] = []
  for (const targetId of targetIds) {
    for (let targetIndex = 0; targetIndex < concurrency; targetIndex += 1) {
      const batchIndex = requests.length + 1
      const image =
        basePayload.mode === 'edit'
          ? inputImages[(batchIndex - 1) % inputImages.length]
          : undefined
      const payload: ImageChannelManualTestParams = {
        ...basePayload,
        batch_id: batchId,
        batch_size: totalRuns,
        batch_index: batchIndex,
        input_image_type: image?.type,
        input_image_name: image?.name,
      }
      requests.push({
        recordId: `${batchId}:${targetId}:${batchIndex}`,
        targetId,
        batchIndex,
        payload,
        ...(image ? { inputImage: image } : {}),
      })
    }
  }
  return requests
}

export interface PollManualRunOptions {
  fetchStatus: () => Promise<ImageChannelManualRunResponse>
  maxAttempts?: number
  maxDurationMs?: number
  now?: () => number
  wait: (attempt: number) => Promise<void>
  retryWait?: (attempt: number) => Promise<void>
  onObservationError?: (error: unknown, attempt: number) => void
}

export class ManualRunObservationError extends Error {
  readonly kind = 'observation_error'
  readonly attempts: number
  readonly cause: unknown

  constructor(message: string, attempts: number, cause?: unknown) {
    super(message)
    this.name = 'ManualRunObservationError'
    this.attempts = attempts
    this.cause = cause
  }
}

const manualRetryableStatuses = new Set([0, 408, 425, 429])

export function isManualRetryableRequestError(error: unknown) {
  if (!error || typeof error !== 'object') return false
  const candidate = error as { status?: number; code?: string; name?: string }
  if (candidate.code === 'ERR_CANCELED' || candidate.name === 'AbortError') return false
  const status = Number(candidate.status)
  return manualRetryableStatuses.has(status) || (status >= 500 && status <= 599)
}

export async function pollManualRunUntilTerminal({
  fetchStatus,
  maxAttempts,
  maxDurationMs,
  now = Date.now,
  wait,
  retryWait,
  onObservationError,
}: PollManualRunOptions): Promise<ImageChannelManualRunResponse> {
  if (maxAttempts !== undefined && (!Number.isSafeInteger(maxAttempts) || maxAttempts < 1)) {
    throw new RangeError('Manual run polling requires at least one attempt')
  }
  if (maxDurationMs !== undefined && (!Number.isFinite(maxDurationMs) || maxDurationMs <= 0)) {
    throw new RangeError('Manual run polling duration must be positive')
  }

  let lastObservationError: unknown
  let consecutiveObservationErrors = 0
  let attempts = 0
  const deadline = maxDurationMs === undefined ? Number.POSITIVE_INFINITY : now() + maxDurationMs
  for (let attempt = 1; maxAttempts === undefined || attempt <= maxAttempts; attempt += 1) {
    attempts = attempt
    let observationFailed = false
    try {
      const run = await fetchStatus()
      consecutiveObservationErrors = 0
      if (!run.running) return run
    } catch (error: unknown) {
      if (!isManualRetryableRequestError(error)) throw error
      lastObservationError = error
      observationFailed = true
      consecutiveObservationErrors += 1
      onObservationError?.(error, attempt)
    }

    if (now() >= deadline) break
    if (maxAttempts === undefined || attempt < maxAttempts) {
      await (observationFailed && retryWait
        ? retryWait(consecutiveObservationErrors)
        : wait(attempt))
    }
  }

  throw new ManualRunObservationError(
    'Manual run did not reach a terminal state within the observation budget',
    attempts,
    lastObservationError
  )
}
