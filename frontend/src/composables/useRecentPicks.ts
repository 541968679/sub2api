/**
 * Persist recently selected filter entities (users/accounts) in localStorage (MRU).
 */

export interface RecentPick {
  id: number
  label: string
  visitedAt: number
}

const DEFAULT_MAX = 15

function safeParse(raw: string | null): RecentPick[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter(
        (item): item is RecentPick =>
          !!item &&
          typeof item === 'object' &&
          typeof (item as RecentPick).id === 'number' &&
          typeof (item as RecentPick).label === 'string'
      )
      .map((item) => ({
        id: item.id,
        label: item.label,
        visitedAt: typeof item.visitedAt === 'number' ? item.visitedAt : 0
      }))
  } catch {
    return []
  }
}

export function loadRecentPicks(storageKey: string): RecentPick[] {
  if (typeof window === 'undefined') return []
  try {
    return safeParse(window.localStorage.getItem(storageKey)).sort(
      (a, b) => b.visitedAt - a.visitedAt
    )
  } catch {
    return []
  }
}

export function pushRecentPick(
  storageKey: string,
  pick: { id: number; label: string },
  max = DEFAULT_MAX
): RecentPick[] {
  if (typeof window === 'undefined') return []
  const now = Date.now()
  const prev = loadRecentPicks(storageKey).filter((p) => p.id !== pick.id)
  const next: RecentPick[] = [{ id: pick.id, label: pick.label, visitedAt: now }, ...prev].slice(
    0,
    max
  )
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(next))
  } catch {
    // ignore quota / private mode
  }
  return next
}

export function clearRecentPicks(storageKey: string): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.removeItem(storageKey)
  } catch {
    // ignore
  }
}
