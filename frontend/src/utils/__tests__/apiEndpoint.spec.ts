import { describe, expect, it } from 'vitest'
import {
  endpointChipName,
  hasHongKongCustomEndpoint,
  resolveUnifiedApiEndpoint,
} from '../apiEndpoint'

describe('resolveUnifiedApiEndpoint', () => {
  it('uses the current site when the admin endpoint is empty', () => {
    expect(resolveUnifiedApiEndpoint('', 'https://zerocode.example.com')).toBe(
      'https://zerocode.example.com'
    )
    expect(resolveUnifiedApiEndpoint('   ', 'https://zerocode.example.com/')).toBe(
      'https://zerocode.example.com'
    )
  })

  it('keeps a configured endpoint and drops a trailing slash', () => {
    expect(
      resolveUnifiedApiEndpoint('https://api.example.com/v1/', 'https://ignored.example.com')
    ).toBe('https://api.example.com/v1')
  })
})

describe('endpointChipName', () => {
  const t = (key: string) =>
    ({
      'keys.endpoints.title': 'API 端点',
      'keys.endpoints.overseas': '海外',
      'keys.endpoints.hongKong': '香港',
    })[key] ?? key

  it('keeps the default title when there is no Hong Kong endpoint', () => {
    expect(
      endpointChipName({ isDefault: true, name: '', hasHongKong: false, t })
    ).toBe('API 端点')
  })

  it('labels the default overseas and the custom row Hong Kong', () => {
    expect(
      endpointChipName({ isDefault: true, name: '', hasHongKong: true, t })
    ).toBe('海外')
    expect(
      endpointChipName({ isDefault: false, name: '香港', hasHongKong: true, t })
    ).toBe('香港')
    expect(
      endpointChipName({ isDefault: false, name: 'Hong Kong', hasHongKong: true, t })
    ).toBe('香港')
  })

  it('detects a Hong Kong custom endpoint by name', () => {
    expect(hasHongKongCustomEndpoint([{ name: '香港' }])).toBe(true)
    expect(hasHongKongCustomEndpoint([{ name: 'OpenAI Compatible' }])).toBe(false)
  })
})
