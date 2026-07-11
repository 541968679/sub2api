import { describe, expect, it } from 'vitest'
import { hasImageOutputTokens, textOutputTokens } from './imageUsage'

describe('image output usage helpers', () => {
  it('separates real image output tokens from total output tokens', () => {
    const row = { output_tokens: 1800, image_output_tokens: 1756 }

    expect(hasImageOutputTokens(row)).toBe(true)
    expect(textOutputTokens(row)).toBe(44)
  })

  it('never fabricates negative text output tokens', () => {
    expect(textOutputTokens({ output_tokens: 10, image_output_tokens: 12 })).toBe(0)
  })
})
