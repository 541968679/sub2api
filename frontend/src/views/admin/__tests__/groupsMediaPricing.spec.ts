import { describe, expect, it } from 'vitest'

import {
  getDefaultImagePreviewPrice,
  getDefaultVideoPreviewPrice,
  getImagePricePlaceholder,
  getVideoPricePlaceholder,
  supportsImagePricingPlatform,
  supportsVideoPricingPlatform
} from '../groupsMediaPricing'

describe('group media pricing platform support', () => {
  it('exposes Grok image and video pricing without exposing video controls elsewhere', () => {
    expect(supportsImagePricingPlatform('grok')).toBe(true)
    expect(supportsVideoPricingPlatform('grok')).toBe(true)
    expect(supportsVideoPricingPlatform('openai')).toBe(false)
    expect(supportsImagePricingPlatform('anthropic')).toBe(false)
  })

  it('uses the current Grok media default rate card', () => {
    expect(getImagePricePlaceholder('grok', 'image_price_1k')).toBe('0.02')
    expect(getImagePricePlaceholder('grok', 'image_price_2k')).toBe('0.02')
    expect(getImagePricePlaceholder('grok', 'image_price_4k')).toBe('0.02')
    expect(getDefaultImagePreviewPrice('grok', 'image_price_1k')).toBe(0.02)

    expect(getVideoPricePlaceholder('grok', 'video_price_480p')).toBe('0.05')
    expect(getVideoPricePlaceholder('grok', 'video_price_720p')).toBe('0.07')
    expect(getVideoPricePlaceholder('grok', 'video_price_1080p')).toBe('0.25')
    expect(getDefaultVideoPreviewPrice('grok', 'video_price_1080p')).toBe(0.25)
    expect(getDefaultVideoPreviewPrice('openai', 'video_price_480p')).toBeNull()
  })
})
