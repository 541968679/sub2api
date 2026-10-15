import type { GroupPlatform } from '@/types'
import { isDomesticAccountGroup, type DomesticAccountGroupHint } from '@/utils/domesticAccountGroups'

export type KeyGroupProvider = 'anthropic' | 'openai' | 'domestic' | 'other'

export const KEY_GROUP_PROVIDERS = ['anthropic', 'openai', 'domestic', 'other'] as const

// Classify by the configured upstream platform, never by a group's display name.
const PROVIDER_BY_PLATFORM: Record<GroupPlatform, KeyGroupProvider> = {
  anthropic: 'anthropic',
  openai: 'openai',
  kimi: 'domestic',
  zhipu: 'domestic',
  deepseek: 'domestic',
  minimax: 'domestic',
  gemini: 'other',
  grok: 'other',
  antigravity: 'other'
}

export function getKeyGroupProvider(platform: GroupPlatform): KeyGroupProvider {
  return PROVIDER_BY_PLATFORM[platform] ?? 'other'
}

// OpenAI-platform groups that sell domestic models stay platform=openai in storage.
// The create-key picker still files them under 国产模型.
export function keyGroupProviderForGroup(group: DomesticAccountGroupHint & { platform?: string | null }): KeyGroupProvider {
  if (isDomesticAccountGroup(group)) return 'domestic'
  return getKeyGroupProvider((group.platform || '') as GroupPlatform)
}

// Collections use representative provider marks rather than an invented brand logo.
export const KEY_GROUP_PROVIDER_ICONS: Record<KeyGroupProvider, GroupPlatform[]> = {
  anthropic: ['anthropic'],
  openai: ['openai'],
  domestic: ['deepseek', 'kimi'],
  other: ['gemini', 'grok']
}
