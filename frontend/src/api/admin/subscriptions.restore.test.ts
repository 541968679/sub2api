import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import { restore, revoke } from './subscriptions'
import type { UserSubscription } from '@/types'

vi.mock('../client', () => ({
  apiClient: {
    post: vi.fn(),
  },
}))

const postMock = vi.mocked(apiClient.post)

afterEach(() => {
  vi.clearAllMocks()
})

describe('admin subscription revoke and restore API', () => {
  it('uses explicit POST action routes', async () => {
    const revoked = { message: 'ok' }
    const restored = { id: 42, status: 'active' } as UserSubscription
    postMock.mockResolvedValueOnce({ data: revoked }).mockResolvedValueOnce({ data: restored })

    await expect(revoke(42)).resolves.toBe(revoked)
    await expect(restore(42)).resolves.toBe(restored)

    expect(postMock).toHaveBeenNthCalledWith(1, '/admin/subscriptions/42/revoke')
    expect(postMock).toHaveBeenNthCalledWith(2, '/admin/subscriptions/42/restore')
  })
})
