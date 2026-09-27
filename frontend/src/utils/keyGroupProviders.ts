import type { GroupPlatform } from '@/types'

export type KeyGroupProvider = 'anthropic' | 'openai' | 'domestic' | 'other'
export type DomesticFamily = 'glm' | 'kimi' | 'deepseek' | 'minimax' | 'mixed'

export const KEY_GROUP_PROVIDERS = ['anthropic', 'openai', 'domestic', 'other'] as const

export interface KeyGroupClassificationInput {
  platform: GroupPlatform
  name?: string | null
  ccs_import_default_model?: string | null
  models_list_config?: { enabled?: boolean; models?: string[] } | null
}

// Platform decides the button. An OpenAI group moves to a domestic family only
// when its enabled custom /v1/models list is entirely that family. Names and
// the CCS import default are not used; both mislabel aggregator groups.
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

const FAMILY_SECTION_PLATFORM: Record<Exclude<DomesticFamily, 'mixed'>, GroupPlatform> = {
  glm: 'zhipu',
  kimi: 'kimi',
  deepseek: 'deepseek',
  minimax: 'minimax'
}

export function getKeyGroupProvider(
  input: GroupPlatform | KeyGroupClassificationInput
): KeyGroupProvider {
  const hint = typeof input === 'string' ? { platform: input } : input
  if (hint.platform === 'openai' && domesticFamilyOf(hint)) return 'domestic'
  return PROVIDER_BY_PLATFORM[hint.platform] ?? 'other'
}

// Section key for an OpenAI-platform group that should leave the OpenAI bucket.
// Native domestic platforms keep their own platform id.
export function domesticSectionPlatform(
  input: KeyGroupClassificationInput
): string | null {
  if (input.platform !== 'openai') return null
  const family = domesticFamilyOf(input)
  if (!family) return null
  if (family === 'mixed') return 'domestic-mixed'
  return FAMILY_SECTION_PLATFORM[family]
}

export function domesticFamilyOf(
  input: KeyGroupClassificationInput
): DomesticFamily | null {
  if (input.platform !== 'openai') return null
  return familyFromEnabledModelList(input.models_list_config)
}

function familyFromModelID(modelID: string): DomesticFamily | null {
  const model = modelID.trim().toLowerCase()
  if (!model) return null
  if (model.startsWith('glm')) return 'glm'
  if (model.startsWith('kimi') || model.startsWith('moonshot')) return 'kimi'
  if (model.startsWith('deepseek')) return 'deepseek'
  if (model.startsWith('minimax')) return 'minimax'
  return null
}

function familyFromEnabledModelList(
  config: KeyGroupClassificationInput['models_list_config']
): DomesticFamily | null {
  if (!config?.enabled || !config.models?.length) return null
  const families = new Set<DomesticFamily>()
  for (const model of config.models) {
    const family = familyFromModelID(model)
    if (!family) return null
    families.add(family)
  }
  if (families.size === 1) return [...families][0]
  if (families.size > 1) return 'mixed'
  return null
}

// Collections use representative provider marks rather than an invented brand logo.
export const KEY_GROUP_PROVIDER_ICONS: Record<KeyGroupProvider, GroupPlatform[]> = {
  anthropic: ['anthropic'],
  openai: ['openai'],
  domestic: ['deepseek', 'kimi'],
  other: ['gemini', 'grok']
}
