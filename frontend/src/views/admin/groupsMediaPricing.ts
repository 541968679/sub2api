const imagePricingPlatforms = new Set(['antigravity', 'gemini', 'grok', 'openai'])

export const supportsImagePricingPlatform = (platform: string): boolean =>
  imagePricingPlatforms.has(platform)

export const supportsVideoPricingPlatform = (platform: string): boolean => platform === 'grok'

type ImagePricingTierKey = 'image_price_1k' | 'image_price_2k' | 'image_price_4k'
type VideoPricingTierKey =
  | 'video_price_480p'
  | 'video_price_720p'
  | 'video_price_1080p'

const defaultImagePrices: Record<string, Record<ImagePricingTierKey, string>> = {
  default: {
    image_price_1k: '0.134',
    image_price_2k: '0.201',
    image_price_4k: '0.268'
  },
  grok: {
    image_price_1k: '0.02',
    image_price_2k: '0.02',
    image_price_4k: '0.02'
  }
}

// Prices are USD per second. 1080p uses the video-1.5 rate because that is the
// Grok model in the current rate card that supports 1080p output.
const defaultVideoPrices: Record<string, Record<VideoPricingTierKey, string>> = {
  grok: {
    video_price_480p: '0.05',
    video_price_720p: '0.07',
    video_price_1080p: '0.25'
  }
}

export const getImagePricePlaceholder = (
  platform: string,
  tier: ImagePricingTierKey
): string => (defaultImagePrices[platform] ?? defaultImagePrices.default)[tier]

export const getVideoPricePlaceholder = (
  platform: string,
  tier: VideoPricingTierKey
): string => defaultVideoPrices[platform]?.[tier] ?? ''

export const getDefaultImagePreviewPrice = (
  platform: string,
  tier: ImagePricingTierKey
): number | null => parseDefaultPrice(getImagePricePlaceholder(platform, tier))

export const getDefaultVideoPreviewPrice = (
  platform: string,
  tier: VideoPricingTierKey
): number | null => parseDefaultPrice(getVideoPricePlaceholder(platform, tier))

const parseDefaultPrice = (value: string): number | null => {
  if (!value) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : null
}
