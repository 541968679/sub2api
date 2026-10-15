import { isDomesticGroupCategory } from '@/utils/domesticGroupCategory'

/** UI label for a group whose gateway platform stays openai. */
export const DOMESTIC_PLATFORM_CHOICE = 'domestic'

export const DOMESTIC_STORED_PLATFORMS = ['kimi', 'zhipu', 'deepseek', 'minimax'] as const

export type DomesticStoredPlatform = (typeof DOMESTIC_STORED_PLATFORMS)[number]

const DOMESTIC_STORED_PLATFORM_SET = new Set<string>(DOMESTIC_STORED_PLATFORMS)

const GROUP_STORED_PLATFORMS = new Set<string>([
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  ...DOMESTIC_STORED_PLATFORMS
])

/**
 * Pay-as-you-go endpoints. A coding-plan URL is typed into the base URL field.
 * These match the backend defaults in domain_constants.go.
 */
export const DOMESTIC_PLATFORM_DEFAULT_BASE_URL: Record<DomesticStoredPlatform, string> = {
  kimi: 'https://api.moonshot.cn/v1',
  zhipu: 'https://open.bigmodel.cn/api/paas/v4',
  deepseek: 'https://api.deepseek.com',
  minimax: 'https://api.minimaxi.com/v1'
}

export function isDomesticStoredPlatform(platform: string | null | undefined): platform is DomesticStoredPlatform {
  return DOMESTIC_STORED_PLATFORM_SET.has((platform || '').trim().toLowerCase())
}

export function domesticPlatformDefaultBaseURL(platform: string | null | undefined): string | undefined {
  const normalized = (platform || '').trim().toLowerCase()
  if (!isDomesticStoredPlatform(normalized)) return undefined
  return DOMESTIC_PLATFORM_DEFAULT_BASE_URL[normalized]
}

/**
 * Platform dropdown value.
 * 国产模型 groups are stored as openai plus a category. Older Kimi/智谱/DeepSeek/MiniMax
 * rows still fold into the same label.
 */
export function groupPlatformChoice(platform: string | null | undefined, category?: string | null): string {
  const normalized = (platform || '').trim().toLowerCase()
  if (normalized === 'openai' && isDomesticGroupCategory(category)) return DOMESTIC_PLATFORM_CHOICE
  if (isDomesticStoredPlatform(normalized)) return DOMESTIC_PLATFORM_CHOICE
  return platform || ''
}

/** Choosing 国产模型 stores openai. The gateway keeps using the OpenAI path. */
export function storedPlatformForGroupChoice(choice: string, current: string): string {
  if (choice === DOMESTIC_PLATFORM_CHOICE) return 'openai'
  if (GROUP_STORED_PLATFORMS.has(choice)) return choice
  return current
}
