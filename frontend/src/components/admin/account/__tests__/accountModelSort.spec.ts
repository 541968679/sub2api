import { describe, expect, it } from 'vitest'
import { sortAccountTestModels } from '../accountModelSort'
import type { ClaudeModel } from '@/types'

function model(id: string, displayName = id): ClaudeModel {
  return {
    id,
    display_name: displayName,
    type: 'model',
    created_at: ''
  }
}

describe('sortAccountTestModels', () => {
  it('places mainstream and newer account test models first', () => {
    const sorted = sortAccountTestModels([
      model('gpt-4o-mini'),
      model('claude-sonnet-4'),
      model('gpt-5.4'),
      model('claude-opus-4-8', 'Opus 4.8'),
      model('gemini-2.5-flash'),
      model('gpt-5.5'),
      model('custom-legacy-model')
    ])

    expect(sorted.map((item) => item.id)).toEqual([
      'claude-opus-4-8',
      'gpt-5.5',
      'gpt-5.4',
      'claude-sonnet-4',
      'gemini-2.5-flash',
      'gpt-4o-mini',
      'custom-legacy-model'
    ])
  })

  it('recognizes compact model spelling such as opus48 and gpt55', () => {
    const sorted = sortAccountTestModels([
      model('gpt54'),
      model('opus48'),
      model('gpt55')
    ])

    expect(sorted.map((item) => item.id)).toEqual(['opus48', 'gpt55', 'gpt54'])
  })
})
