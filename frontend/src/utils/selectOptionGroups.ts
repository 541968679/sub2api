export const GROUP_PLATFORM_ORDER = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek',
  'minimax'
] as const

export interface PlatformSectionHeader {
  kind: 'group'
  value: string
  label: string
  disabled: true
  platform: string
}

export interface PlatformSection<T> {
  platform: string
  label: string
  items: T[]
}

export function isSelectGroupHeader(option: unknown): option is PlatformSectionHeader {
  return !!option && typeof option === 'object' && (option as { kind?: unknown }).kind === 'group'
}

export function sectionsByPlatform<T extends { platform?: string | null }>(
  items: readonly T[],
  platformLabel: (platform: string) => string
): PlatformSection<T>[] {
  const buckets = new Map<string, T[]>()
  for (const item of items) {
    const key = item.platform || 'other'
    const bucket = buckets.get(key)
    if (bucket) bucket.push(item)
    else buckets.set(key, [item])
  }

  const known = new Set<string>(GROUP_PLATFORM_ORDER)
  const keys = [
    ...GROUP_PLATFORM_ORDER.filter((platform) => buckets.has(platform)),
    ...[...buckets.keys()].filter((platform) => !known.has(platform))
  ]

  return keys.map((platform) => ({
    platform,
    label: platformLabel(platform),
    items: buckets.get(platform) ?? []
  }))
}

export function flattenPlatformSections<T>(
  sections: readonly PlatformSection<T>[]
): Array<PlatformSectionHeader | T> {
  const rows: Array<PlatformSectionHeader | T> = []
  for (const section of sections) {
    if (!section.items.length) continue
    rows.push({
      kind: 'group',
      value: `platform:${section.platform}`,
      label: section.label,
      disabled: true,
      platform: section.platform
    })
    rows.push(...section.items)
  }
  return rows
}

interface FilterAccessors<T> {
  label?: (option: T) => string
  description?: (option: T) => unknown
}

function optionHaystack<T>(option: T, accessors?: FilterAccessors<T>): string {
  if (option === null || typeof option !== 'object') return String(option ?? '').toLowerCase()
  const record = option as Record<string, unknown>
  const label = accessors?.label ? accessors.label(option) : String(record.label ?? '')
  const descriptionSource = accessors?.description ? accessors.description(option) : record.description
  const description =
    descriptionSource === null || descriptionSource === undefined ? '' : String(descriptionSource)
  const platform = typeof record.platform === 'string' ? record.platform : ''
  return `${label}\n${description}\n${platform}`.toLowerCase()
}

/**
 * Keeps platform headings attached to the groups that still match.
 * A heading match (platform name or id) keeps every group in that section.
 */
export function filterOptionsPreservingGroups<T>(
  options: readonly T[],
  query: string,
  accessors?: FilterAccessors<T>
): T[] {
  const q = query.trim().toLowerCase()
  if (!q) return [...options]

  const matches = (option: T) => optionHaystack(option, accessors).includes(q)
  const result: T[] = []
  let index = 0
  while (index < options.length) {
    const current = options[index]
    if (!isSelectGroupHeader(current)) {
      if (matches(current)) result.push(current)
      index += 1
      continue
    }

    const header = current
    index += 1
    const children: T[] = []
    while (index < options.length && !isSelectGroupHeader(options[index])) {
      children.push(options[index])
      index += 1
    }
    if (!children.length) continue
    if (matches(header as T)) {
      result.push(header as T, ...children)
      continue
    }
    const matched = children.filter(matches)
    if (matched.length) result.push(header as T, ...matched)
  }
  return result
}

export function sectionsFromGroupedOptions<T>(
  options: readonly (T | PlatformSectionHeader)[]
): PlatformSection<T>[] {
  const sections: PlatformSection<T>[] = []
  let current: PlatformSection<T> | null = null
  for (const option of options) {
    if (isSelectGroupHeader(option)) {
      current = {
        platform: option.platform,
        label: option.label,
        items: []
      }
      sections.push(current)
      continue
    }
    if (current) current.items.push(option)
  }
  return sections.filter((section) => section.items.length > 0)
}
