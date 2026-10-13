export interface GroupSortItem {
  id: number
  sort_order?: number | null
}

function sortRank(group: GroupSortItem): number {
  return typeof group.sort_order === 'number' && Number.isFinite(group.sort_order) ? group.sort_order : 0
}

/** Same order as `GET /admin/groups/all`: sort_order ascending, then id. */
export function compareAdminGroupsBySort(a: GroupSortItem, b: GroupSortItem): number {
  const rank = sortRank(a) - sortRank(b)
  if (rank !== 0) return rank
  return a.id - b.id
}

/**
 * Splice each reordered subset back into the current display order.
 * A subset only moves its own groups; groups outside every subset keep their slots.
 * The result renumbers every known group so ties on sort_order 0 become an explicit order.
 */
export function mergeGroupSortOrders(
  groups: GroupSortItem[],
  reorderedSubsets: number[][]
): Array<{ id: number; sort_order: number }> {
  const sorted = [...groups].sort(compareAdminGroupsBySort)
  const known = new Set(sorted.map((group) => group.id))
  const queues = reorderedSubsets.map((ids) => {
    const unique: number[] = []
    const seen = new Set<number>()
    for (const id of ids) {
      if (!known.has(id) || seen.has(id)) continue
      seen.add(id)
      unique.push(id)
    }
    return { set: seen, queue: unique }
  })

  const emitted = new Set<number>()
  const next: number[] = []
  const push = (id: number) => {
    if (emitted.has(id)) return
    emitted.add(id)
    next.push(id)
  }

  for (const group of sorted) {
    const bucket = queues.find((item) => item.set.has(group.id))
    if (!bucket) {
      push(group.id)
      continue
    }
    push(bucket.queue.shift() ?? group.id)
  }

  for (const bucket of queues) {
    for (const id of bucket.queue) push(id)
  }

  return next.map((id, index) => ({ id, sort_order: (index + 1) * 10 }))
}
