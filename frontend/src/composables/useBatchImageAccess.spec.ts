import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { list } = vi.hoisted(() => ({ list: vi.fn() }))
vi.mock('@/api/keys', () => ({ keysAPI: { list } }))

import { useAuthStore } from '@/stores/auth'
import { useBatchImageAccess } from './useBatchImageAccess'

describe('useBatchImageAccess', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    list.mockReset()
    const auth = useAuthStore()
    auth.token = 'test-token'
    auth.user = { id: 1, role: 'user' } as any
  })

  it('allows an active key assigned to an enabled Gemini group', async () => {
    list.mockResolvedValue({
      items: [{ status: 'active', group: { platform: 'gemini', allow_batch_image_generation: true } }],
      pages: 1,
    })
    const access = useBatchImageAccess()
    expect(await access.refreshBatchImageAccess(true)).toBe(true)
    expect(access.canUseBatchImage.value).toBe(true)
  })

  it('rejects disabled, non-Gemini, and unprivileged keys', async () => {
    list.mockResolvedValue({
      items: [
        { status: 'inactive', group: { platform: 'gemini', allow_batch_image_generation: true } },
        { status: 'active', group: { platform: 'openai', allow_batch_image_generation: true } },
        { status: 'active', group: { platform: 'gemini', allow_batch_image_generation: false } },
      ],
      pages: 1,
    })
    const access = useBatchImageAccess()
    expect(await access.refreshBatchImageAccess(true)).toBe(false)
    expect(access.canUseBatchImage.value).toBe(false)
  })
})
