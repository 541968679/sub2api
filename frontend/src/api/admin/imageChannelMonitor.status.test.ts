import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import { getManualTestStatus } from './imageChannelMonitor'
import type { ImageChannelManualRunResponse } from './imageChannelMonitor'

vi.mock('../client', () => ({
  apiClient: {
    get: vi.fn(),
  },
}))

const getMock = vi.mocked(apiClient.get)

afterEach(() => {
  vi.clearAllMocks()
})

describe('manual image test status API', () => {
  it('uses a permanently metadata-only status response', async () => {
    const status = {
      run_id: 'run-1',
      running: true,
      result: undefined,
    } as ImageChannelManualRunResponse
    getMock.mockResolvedValueOnce({ data: status })

    await expect(getManualTestStatus(42, 'run-1', { timeoutSeconds: 600 })).resolves.toBe(status)

    expect(getMock).toHaveBeenCalledTimes(1)
    expect(getMock.mock.calls[0]?.[0]).toBe('/admin/image-channel-monitors/42/manual-test/run-1')
    const config = getMock.mock.calls[0]?.[1]
    expect(config?.params ?? {}).not.toHaveProperty('include_image_data')
    expect(config?.timeout).toBe(15_000)
  })
})
