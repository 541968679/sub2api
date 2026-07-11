import { describe, expect, it, vi } from 'vitest'

import {
  buildManualRunRequests,
  isManualRetryableRequestError,
  manualArtifactRecoveryExpiresAt,
  manualRunObservationMaxAttempts,
  pollManualRunUntilTerminal,
  revokeManualObjectURLsForRun,
} from './imageChannelManualTest'
import type {
  ImageChannelManualRunResponse,
  ImageChannelManualTestParams,
} from '@/api/admin/imageChannelMonitor'

describe('manual image test polling', () => {
  it('only applies a calculable observation deadline to direct probes', () => {
    expect(manualRunObservationMaxAttempts('direct_probe', 600)).toBe(645)
    expect(manualRunObservationMaxAttempts('gateway_account', 600)).toBeUndefined()
    expect(manualRunObservationMaxAttempts('gateway_group', 30)).toBeUndefined()
  })

  it('recovers from an observation network error without turning it into an image failure', async () => {
    const running = { run_id: 'run-1', running: true } as ImageChannelManualRunResponse
    const terminal = {
      run_id: 'run-1',
      running: false,
      result: { status: 'operational' },
    } as ImageChannelManualRunResponse
    const networkError = {
      status: 0,
      code: 'ERR_NETWORK',
      reason: 'socket disconnected before response headers',
    }
    const fetchStatus = vi
      .fn<() => Promise<ImageChannelManualRunResponse>>()
      .mockRejectedValueOnce(networkError)
      .mockResolvedValueOnce(running)
      .mockResolvedValueOnce(terminal)
    const onObservationError = vi.fn()

    const result = await pollManualRunUntilTerminal({
      fetchStatus,
      maxAttempts: 3,
      wait: async () => undefined,
      onObservationError,
    })

    expect(result).toBe(terminal)
    expect(fetchStatus).toHaveBeenCalledTimes(3)
    expect(onObservationError).toHaveBeenCalledWith(networkError, 1)
  })

  it('enforces the observation budget by wall-clock time when status requests are slow', async () => {
    let nowMs = 0
    const running = { run_id: 'run-slow-control', running: true } as ImageChannelManualRunResponse
    const fetchStatus = vi.fn(async () => {
      nowMs += 15_000
      return running
    })
    const wait = vi.fn(async () => {
      nowMs += 1_000
    })

    await expect(
      pollManualRunUntilTerminal({
        fetchStatus,
        maxAttempts: 100,
        maxDurationMs: 31_000,
        now: () => nowMs,
        wait,
      })
    ).rejects.toMatchObject({
      name: 'ManualRunObservationError',
      attempts: 2,
    })

    expect(fetchStatus).toHaveBeenCalledTimes(2)
    expect(wait).toHaveBeenCalledTimes(1)
  })

  it('recovers from a retryable HTTP observation error', async () => {
    const terminal = {
      run_id: 'run-503',
      running: false,
      result: { status: 'operational' },
    } as ImageChannelManualRunResponse
    const unavailable = { status: 503, code: 'SERVICE_UNAVAILABLE' }
    const fetchStatus = vi
      .fn<() => Promise<ImageChannelManualRunResponse>>()
      .mockRejectedValueOnce(unavailable)
      .mockResolvedValueOnce(terminal)
    const onObservationError = vi.fn()

    await expect(
      pollManualRunUntilTerminal({
        fetchStatus,
        maxAttempts: 2,
        wait: async () => undefined,
        onObservationError,
      })
    ).resolves.toBe(terminal)

    expect(onObservationError).toHaveBeenCalledWith(unavailable, 1)
  })

  it('rethrows an HTTP observation error immediately instead of polling through it', async () => {
    const notFound = { status: 404, code: 'MANUAL_RUN_NOT_FOUND' }
    const fetchStatus = vi.fn<() => Promise<ImageChannelManualRunResponse>>()
      .mockRejectedValue(notFound)
    const wait = vi.fn(async () => undefined)
    const onObservationError = vi.fn()

    await expect(
      pollManualRunUntilTerminal({
        fetchStatus,
        maxAttempts: 3,
        wait,
        onObservationError,
      })
    ).rejects.toBe(notFound)

    expect(fetchStatus).toHaveBeenCalledTimes(1)
    expect(wait).not.toHaveBeenCalled()
    expect(onObservationError).not.toHaveBeenCalled()
  })
})

describe('manual artifact recovery deadline', () => {
  it('uses the backend 30 minute retention window from task completion', () => {
    expect(manualArtifactRecoveryExpiresAt('2026-07-10T10:00:01.000Z')).toBe(
      '2026-07-10T10:30:01.000Z'
    )
  })
})

describe('manual image object URL cleanup', () => {
  it('revokes live and persisted preview URLs for the same run', () => {
    const urls = new Map([
      ['run-1', 'blob:live-output'],
      ['run-1:input', 'blob:stored-input'],
      ['run-1:output', 'blob:stored-output'],
      ['run-2:output', 'blob:other-run'],
    ])
    const revoke = vi.fn()

    revokeManualObjectURLsForRun(urls, 'run-1', revoke)

    expect(revoke.mock.calls.flat()).toEqual([
      'blob:live-output',
      'blob:stored-input',
      'blob:stored-output',
    ])
    expect([...urls.entries()]).toEqual([['run-2:output', 'blob:other-run']])
  })
})

describe('manual request retry classification', () => {
  it.each([0, 408, 425, 429, 500, 502, 503, 504])('retries status %s', (status) => {
    expect(isManualRetryableRequestError({ status })).toBe(true)
  })

  it.each([404, 409, 410])('does not retry terminal status %s', (status) => {
    expect(isManualRetryableRequestError({ status })).toBe(false)
  })

  it.each([
    { status: 0, code: 'ERR_CANCELED' },
    { status: 0, name: 'AbortError' },
  ])('does not retry an intentional cancellation %#', (error) => {
    expect(isManualRetryableRequestError(error)).toBe(false)
  })
})

describe('manual edit request construction', () => {
  it('builds cN as N independent requests with N distinct multipart image descriptors', () => {
    const images = [
      { data: 'data:image/png;base64,AAAA', type: 'image/png', name: 'one.png' },
      { data: 'data:image/jpeg;base64,BBBB', type: 'image/jpeg', name: 'two.jpg' },
      { data: 'data:image/webp;base64,CCCC', type: 'image/webp', name: 'three.webp' },
    ]
    const basePayload: ImageChannelManualTestParams = {
      mode: 'edit',
      model: 'gpt-image-1',
      prompt: 'edit independently',
      n: 1,
    }

    const requests = buildManualRunRequests({
      targetIds: [42],
      concurrency: 3,
      batchId: 'ui-correlation-only',
      basePayload,
      inputImages: images,
    })

    expect(requests).toHaveLength(3)
    expect(requests.map((request) => request.targetId)).toEqual([42, 42, 42])
    expect(requests.map((request) => request.payload.input_image_data)).toEqual([undefined, undefined, undefined])
    expect(requests.map((request) => request.payload.input_image_name)).toEqual(
      images.map((image) => image.name)
    )
    expect(requests.map((request) => request.inputImage)).toEqual(images)
    expect(new Set(requests.map((request) => request.payload)).size).toBe(3)
    expect(requests.every((request) => request.payload.mode === 'edit')).toBe(true)
  })

  it('reuses a smaller input pool across concurrent edit requests in round-robin order', () => {
    const images = [
      { data: 'data:image/png;base64,AAAA', type: 'image/png', name: 'one.png' },
      { data: 'data:image/jpeg;base64,BBBB', type: 'image/jpeg', name: 'two.jpg' },
    ]

    const requests = buildManualRunRequests({
      targetIds: [42],
      concurrency: 5,
      batchId: 'ui-correlation-only',
      basePayload: { mode: 'edit', prompt: 'reuse pool' },
      inputImages: images,
    })

    expect(requests.map((request) => request.payload.input_image_name)).toEqual([
      'one.png',
      'two.jpg',
      'one.png',
      'two.jpg',
      'one.png',
    ])
    expect(requests.map((request) => request.inputImage)).toEqual([
      images[0],
      images[1],
      images[0],
      images[1],
      images[0],
    ])
    expect(new Set(requests.map((request) => request.payload)).size).toBe(5)
  })

  it('sends the same single image on every concurrent edit request', () => {
    const image = { data: 'data:image/png;base64,AAAA', type: 'image/png', name: 'only-one.png' }

    const requests = buildManualRunRequests({
      targetIds: [42],
      concurrency: 3,
      batchId: 'ui-correlation-only',
      basePayload: { mode: 'edit', prompt: 'single image reuse' },
      inputImages: [image],
    })

    expect(requests).toHaveLength(3)
    expect(requests.every((request) => request.inputImage === image)).toBe(true)
    expect(requests.every((request) => request.payload.input_image_name === 'only-one.png')).toBe(
      true
    )
  })

  it('rejects edit runs when the input pool is empty', () => {
    expect(() =>
      buildManualRunRequests({
        targetIds: [42],
        concurrency: 2,
        batchId: 'ui-correlation-only',
        basePayload: { mode: 'edit', prompt: 'needs an input' },
        inputImages: [],
      })
    ).toThrow(/at least one input image/i)
  })
})
