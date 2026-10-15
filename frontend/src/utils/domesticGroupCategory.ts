export const DOMESTIC_GROUP_CATEGORIES = ['mixed', 'deepseek', 'kimi', 'glm', 'qwen'] as const

export type DomesticGroupCategory = (typeof DOMESTIC_GROUP_CATEGORIES)[number]

const CATEGORY_SET = new Set<string>(DOMESTIC_GROUP_CATEGORIES)

export interface DomesticGroupCategoryHint {
  domestic_category?: string | null
  name?: string | null
  description?: string | null
}

export function isDomesticGroupCategory(value: string | null | undefined): value is DomesticGroupCategory {
  return CATEGORY_SET.has((value || '').trim().toLowerCase())
}

/** Explicit category wins. Otherwise the group name picks one category, or 混合. */
export function resolveDomesticGroupCategory(group: DomesticGroupCategoryHint): DomesticGroupCategory {
  const explicit = (group.domestic_category || '').trim().toLowerCase()
  if (isDomesticGroupCategory(explicit)) return explicit

  const label = `${group.name || ''}\n${group.description || ''}`.toLowerCase()
  if (label.includes('deepseek')) return 'deepseek'
  if (label.includes('kimi') || label.includes('moonshot') || label.includes('月之暗面')) return 'kimi'
  if (label.includes('glm') || label.includes('智谱') || label.includes('chatglm') || label.includes('zhipu')) return 'glm'
  if (label.includes('qwen') || label.includes('通义') || label.includes('千问') || label.includes('百炼')) return 'qwen'
  return 'mixed'
}
