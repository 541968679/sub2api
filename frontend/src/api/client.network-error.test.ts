import { afterEach, describe, expect, it } from 'vitest'
import { AxiosError, type InternalAxiosRequestConfig } from 'axios'

import { apiClient } from './client'

const originalAdapter = apiClient.defaults.adapter

afterEach(() => {
  apiClient.defaults.adapter = originalAdapter
})

describe('apiClient network error details', () => {
  it.each([
    ['ECONNABORTED', 'timeout of 45000ms exceeded'],
    ['ERR_NETWORK', 'socket disconnected before response headers'],
  ])('preserves Axios code %s and its original reason', async (code, reason) => {
    apiClient.defaults.adapter = async (config) => {
      throw new AxiosError(reason, code, config as InternalAxiosRequestConfig)
    }

    await expect(apiClient.get('/network-failure')).rejects.toMatchObject({
      status: 0,
      code,
      reason,
      message: 'Network error. Please check your connection.',
    })
  })
})
