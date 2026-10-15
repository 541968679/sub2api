import { describe, expect, it } from 'vitest'
import {
  API_KEY_IMPORT_MAX,
  assignAPIKeyImportNames,
  parseAPIKeyImport,
  platformDefaultAPIKeyBaseURL,
  resolveAPIKeyImportBaseURL,
  TOKENRHYTHM_ORIGIN
} from '../apikeyBatchParse'

describe('parseAPIKeyImport', () => {
  it('accepts one plain key per line', () => {
    const parsed = parseAPIKeyImport('sk-one\n\nsk-two')
    expect(parsed.errors).toEqual([])
    expect(parsed.entries.map((entry) => entry.apiKey)).toEqual(['sk-one', 'sk-two'])
  })

  it('parses session, key, and ignores the trailing balance hint', () => {
    const parsed = parseAPIKeyImport(
      'sess_session----sk_tr_secret-key----76.0'
    )
    expect(parsed.errors).toEqual([])
    expect(parsed.entries).toEqual([
      {
        apiKey: 'sk_tr_secret-key',
        sessionToken: 'sess_session',
        line: 1
      }
    ])
  })

  it('parses the longer vendor line in any field order', () => {
    const parsed = parseAPIKeyImport(
      '13800138000|rf_refresh|sk-live|sess_session|123e4567-e89b-12d3-a456-426614174000'
    )
    expect(parsed.errors).toEqual([])
    expect(parsed.entries[0]).toMatchObject({
      apiKey: 'sk-live',
      sessionToken: 'sess_session',
      refreshToken: 'rf_refresh'
    })
  })

  it('reports a duplicate key and a line with no key', () => {
    const parsed = parseAPIKeyImport('sk-one\nsk-one\nsess_only----76.0')
    expect(parsed.entries.map((entry) => entry.apiKey)).toEqual(['sk-one'])
    expect(parsed.errors).toEqual([
      { line: 2, reason: 'duplicate' },
      { line: 3, reason: 'missing_key' }
    ])
  })

  it('uses the tokenrhythm origin when a session line keeps the platform default base URL', () => {
    expect(resolveAPIKeyImportBaseURL('https://api.openai.com', 'https://api.openai.com', true)).toBe(
      TOKENRHYTHM_ORIGIN
    )
    expect(resolveAPIKeyImportBaseURL('https://example.test/v1', 'https://api.openai.com', true)).toBe(
      'https://example.test/v1'
    )
    expect(resolveAPIKeyImportBaseURL('', 'https://api.openai.com', false)).toBe('https://api.openai.com')
    expect(platformDefaultAPIKeyBaseURL('openai')).toBe('https://api.openai.com')
    expect(platformDefaultAPIKeyBaseURL('gemini')).toBe('https://generativelanguage.googleapis.com')
    expect(platformDefaultAPIKeyBaseURL('kimi')).toBe('https://api.moonshot.cn/v1')
    expect(platformDefaultAPIKeyBaseURL('minimax')).toBe('https://api.minimaxi.com/v1')
  })

  it('names batch accounts from the prefix and key tail', () => {
    expect(assignAPIKeyImportNames('基元律动', ['sk_tr_aaaa', 'sk_tr_bbbb'])).toEqual([
      '基元律动-aaaa',
      '基元律动-bbbb'
    ])
    expect(API_KEY_IMPORT_MAX).toBe(500)
  })
})
