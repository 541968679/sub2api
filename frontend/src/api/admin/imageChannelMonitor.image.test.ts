import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import {
  cancelManualTestByClientRunID,
  getManualTestImage,
  manualTest,
} from './imageChannelMonitor'

vi.mock('../client', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

const getMock = vi.mocked(apiClient.get)
const postMock = vi.mocked(apiClient.post)

afterEach(() => {
  vi.clearAllMocks()
})

describe('manual image test result API', () => {
  it('starts edit runs with an independent binary multipart image', async () => {
    postMock.mockResolvedValueOnce({ data: { run_id: 'edit-run-1' } })
    const image = new Blob(['source-image'], { type: 'image/png' })
    const params = {
      mode: 'edit' as const,
      client_run_id: 'client-edit-1',
      input_image_name: 'source.png',
      input_image_type: 'image/png',
    }

    await manualTest(42, params, image)

    const body = postMock.mock.calls[0]?.[1]
    expect(body).toBeInstanceOf(FormData)
    expect((body as FormData).get('image')).toBeInstanceOf(Blob)
    expect(JSON.parse(String((body as FormData).get('metadata')))).toEqual(params)
    expect(postMock.mock.calls[0]?.[2]).toEqual(
      expect.objectContaining({
        timeout: 15_000,
        // Without this override the client-wide application/json default makes
        // axios rewrite FormData into a JSON body that drops every real field.
        headers: { 'Content-Type': 'multipart/form-data' },
      })
    )
  })

  it('keeps generate runs as plain JSON payloads without a multipart override', async () => {
    postMock.mockResolvedValueOnce({ data: { run_id: 'generate-run-1' } })
    const params = { mode: 'generate' as const, client_run_id: 'client-generate-1' }

    await manualTest(42, params)

    expect(postMock.mock.calls[0]?.[1]).toBe(params)
    expect(postMock.mock.calls[0]?.[2]).toEqual({ timeout: 15_000 })
  })

  it('records cancellation intent by client run id without replaying the launch', async () => {
    postMock.mockResolvedValueOnce({ data: undefined })

    await cancelManualTestByClientRunID(42, 'client/run with spaces')

    expect(postMock).toHaveBeenCalledWith(
      '/admin/image-channel-monitors/42/manual-test/client-runs/client%2Frun%20with%20spaces/cancel',
      undefined,
      expect.objectContaining({ timeout: 15_000 })
    )
  })

  it('fetches generated image bytes separately from run status', async () => {
    const image = new Blob(['image-bytes'], { type: 'image/png' })
    getMock.mockResolvedValueOnce({ data: image })

    await expect(getManualTestImage(42, 'run/with spaces')).resolves.toBe(image)

    expect(getMock).toHaveBeenCalledWith(
      '/admin/image-channel-monitors/42/manual-test/run%2Fwith%20spaces/images/0',
      expect.objectContaining({ responseType: 'blob' })
    )
  })

  it('fetches a requested image index without sharing a batch result', async () => {
    const image = new Blob(['second-image'], { type: 'image/webp' })
    getMock.mockResolvedValueOnce({ data: image })

    await expect(getManualTestImage(42, 'run-2', 1)).resolves.toBe(image)

    expect(getMock).toHaveBeenCalledWith(
      '/admin/image-channel-monitors/42/manual-test/run-2/images/1',
      expect.objectContaining({ responseType: 'blob' })
    )
  })

  it('uses the manual run timeout budget instead of the global 30 second timeout', async () => {
    const image = new Blob(['slow-image'], { type: 'image/png' })
    getMock.mockResolvedValueOnce({ data: image })

    await expect(
      getManualTestImage(42, 'run-slow', 0, { timeoutSeconds: 600 })
    ).resolves.toBe(image)

    expect(getMock).toHaveBeenCalledWith(
      '/admin/image-channel-monitors/42/manual-test/run-slow/images/0',
      expect.objectContaining({
        responseType: 'blob',
        timeout: 645_000,
      })
    )
  })
})
