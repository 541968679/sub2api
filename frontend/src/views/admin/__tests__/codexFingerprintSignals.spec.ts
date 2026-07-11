import { describe, expect, it } from 'vitest'
import {
  defaultFingerprintSignalRows,
  parseFingerprintSignalsToRows,
  serializeFingerprintRowsToJSON
} from '../codexFingerprintSignals'

describe('Codex fingerprint signal editor', () => {
  it('keeps an unconfigured policy disabled', () => {
    expect(parseFingerprintSignalsToRows('')).toEqual([])
    expect(serializeFingerprintRowsToJSON([])).toBe('')
  })

  it('round trips required signal variants', () => {
    const raw = serializeFingerprintRowsToJSON([
      { type: 'header_exact', match: 'session-id / session_id', required: true }
    ])
    expect(parseFingerprintSignalsToRows(raw)).toEqual([
      { type: 'header_exact', match: 'session-id / session_id', required: true }
    ])
  })

  it('exposes recommended signals only as an explicit preset', () => {
    expect(defaultFingerprintSignalRows()[0]).toEqual({
      type: 'header_prefix',
      match: 'x-codex-',
      required: true
    })
  })
})
