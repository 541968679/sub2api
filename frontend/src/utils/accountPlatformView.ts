import { isDomesticAccountGroup, type DomesticAccountGroupHint } from '@/utils/domesticAccountGroups'

/** Account-list tab only. It is not stored on accounts and is not sent as platform. */
export const DOMESTIC_ACCOUNT_PLATFORM_VIEW = 'domestic'

export interface AccountPlatformViewGroup extends DomesticAccountGroupHint {
  id: number
}

export interface AccountPlatformViewFilters {
  platform?: string
  type?: string
  status?: string
  group?: string
  search?: string
  privacy_mode?: string
  lite?: string
  include_scheduler_score?: string
  ids?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  group_ids?: string
  exclude_group_ids?: string
}

export function domesticAccountGroupIDs(groups: readonly AccountPlatformViewGroup[]): number[] {
  const ids: number[] = []
  const seen = new Set<number>()
  for (const group of groups) {
    if (!isDomesticAccountGroup(group) || !Number.isInteger(group.id) || group.id <= 0 || seen.has(group.id)) continue
    seen.add(group.id)
    ids.push(group.id)
  }
  return ids
}

function omitGroupSet(filters: AccountPlatformViewFilters): AccountPlatformViewFilters {
  const next = { ...filters }
  delete next.group_ids
  delete next.exclude_group_ids
  return next
}

/**
 * Turn the 国产模型 tab into group membership filters.
 * A specific group uses the existing group query. "All" on that tab asks for
 * those group ids. The OpenAI tab's "all groups" excludes the same ids.
 * Stored account.platform is left unchanged.
 */
export function accountListQueryForPlatformView(
  filters: AccountPlatformViewFilters,
  groups: readonly AccountPlatformViewGroup[]
): AccountPlatformViewFilters {
  const platform = filters.platform || ''
  const group = String(filters.group || '')
  const ids = domesticAccountGroupIDs(groups)
  const idList = ids.length > 0 ? ids.join(',') : '0'

  if (platform === DOMESTIC_ACCOUNT_PLATFORM_VIEW) {
    const next = omitGroupSet(filters)
    delete next.platform
    if (group && group !== 'ungrouped') {
      return { ...next, group }
    }
    return { ...next, group: '', group_ids: idList }
  }

  if (platform === 'openai' && !group) {
    if (ids.length === 0) return omitGroupSet(filters)
    return { ...omitGroupSet(filters), exclude_group_ids: ids.join(',') }
  }

  return omitGroupSet(filters)
}
