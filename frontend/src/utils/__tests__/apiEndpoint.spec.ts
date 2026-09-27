import { describe, expect, it } from 'vitest'
import { resolveUnifiedApiEndpoint } from '../apiEndpoint'

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
