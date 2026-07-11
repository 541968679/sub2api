import { describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => {
      const messages: Record<string, string> = {
        'admin.accounts.oauth.grok.failedToGenerateUrl': 'generate failed',
        'admin.accounts.oauth.grok.failedToExchangeCode': 'exchange failed',
        'admin.accounts.oauth.grok.failedToValidateRT': 'refresh failed',
        'admin.accounts.oauth.grok.errors.GROK_OAUTH_INVALID_STATE': 'state recovery hint',
        'admin.accounts.oauth.grok.errors.GROK_OAUTH_SESSION_NOT_FOUND': 'session recovery hint',
        'admin.accounts.oauth.grok.errors.GROK_OAUTH_NO_REFRESH_TOKEN': 'refresh token recovery hint'
      }
      return messages[key] ?? key
    }
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    grok: {
      generateAuthUrl: vi.fn(),
      exchangeCode: vi.fn(),
      refreshGrokToken: vi.fn()
    }
  }
}))

import { useGrokOAuth } from '@/composables/useGrokOAuth'
import { adminAPI } from '@/api/admin'

describe('useGrokOAuth', () => {
  it('shows a state mismatch recovery hint from structured backend errors', async () => {
    vi.mocked(adminAPI.grok.exchangeCode).mockRejectedValueOnce({
      status: 400,
      reason: 'GROK_OAUTH_INVALID_STATE',
      message: 'invalid oauth state'
    })
    const oauth = useGrokOAuth()

    const tokenInfo = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session-id',
      state: 'wrong-state'
    })

    expect(tokenInfo).toBeNull()
    expect(oauth.error.value).toBe('state recovery hint')
  })

  it('localizes session and refresh-token structured errors', async () => {
    vi.mocked(adminAPI.grok.exchangeCode).mockRejectedValueOnce({
      status: 400,
      reason: 'GROK_OAUTH_SESSION_NOT_FOUND',
      message: 'session missing'
    })
    vi.mocked(adminAPI.grok.refreshGrokToken).mockRejectedValueOnce({
      status: 400,
      reason: 'GROK_OAUTH_NO_REFRESH_TOKEN',
      message: 'refresh token missing'
    })
    const oauth = useGrokOAuth()

    expect(await oauth.exchangeAuthCode({ code: 'code', sessionId: 'expired', state: 'state' })).toBeNull()
    expect(oauth.error.value).toBe('session recovery hint')

    expect(await oauth.validateRefreshToken('refresh-token')).toBeNull()
    expect(oauth.error.value).toBe('refresh token recovery hint')
  })

  it('extracts plain API messages when generating an auth URL fails', async () => {
    vi.mocked(adminAPI.grok.generateAuthUrl).mockRejectedValueOnce({
      status: 502,
      message: 'proxy unavailable'
    })
    const oauth = useGrokOAuth()

    expect(await oauth.generateAuthUrl()).toBe(false)
    expect(oauth.error.value).toBe('proxy unavailable')
  })
})
