import { isDomesticPricingModel } from '@/utils/pricingPageModels'
import { isDomesticGroupCategory } from '@/utils/domesticGroupCategory'

/**
 * OpenAI-platform groups that sell domestic coding models share `platform=openai`
 * with GPT groups. Name tokens below are the ones used on those groups
 * (国产 / 国模 / kimi / glm / deepseek / qwen, plus the same vendors on a
 * dedicated platform). Model ids cover a group whose name does not say so.
 */
const DOMESTIC_GROUP_PLATFORMS = new Set(['kimi', 'zhipu', 'deepseek', 'minimax'])

const DOMESTIC_NAME_TOKENS = [
  '国产',
  '国模',
  '智谱',
  '月之暗面',
  '通义',
  '千问',
  '百炼',
  '豆包',
  'kimi',
  'glm',
  'deepseek',
  'minimax',
  'qwen',
  'moonshot',
  'doubao',
  'mimo',
  'chatglm'
]

export interface DomesticAccountGroupHint {
  platform?: string | null
  name?: string | null
  description?: string | null
  domestic_category?: string | null
  default_mapped_model?: string | null
  ccs_import_default_model?: string | null
  allowed_models?: readonly string[] | null
}

export function isDomesticAccountGroup(group: DomesticAccountGroupHint): boolean {
  if (isDomesticGroupCategory(group.domestic_category)) return true
  const platform = (group.platform || '').trim().toLowerCase()
  if (DOMESTIC_GROUP_PLATFORMS.has(platform)) return true
  if (platform && platform !== 'openai') return false

  const label = `${group.name || ''}\n${group.description || ''}`.toLowerCase()
  if (DOMESTIC_NAME_TOKENS.some((token) => label.includes(token))) return true

  const models = [
    group.default_mapped_model,
    group.ccs_import_default_model,
    ...(group.allowed_models || [])
  ]
  return models.some((model) => isDomesticPricingModel(model))
}
