/**
 * Account list column layout helpers (order + widths).
 * Pure functions so layout merge can be unit-tested without mounting AccountsView.
 */

export const ACCOUNT_COLUMN_LAYOUT_KEY = 'account-column-layout'
export const ACCOUNT_COLUMN_LAYOUT_VERSION = 1

export const PINNED_START_COLUMNS = ['select', 'name'] as const
export const PINNED_END_COLUMNS = ['actions'] as const

export const MIN_COLUMN_WIDTH = 64
export const MIN_SELECT_COLUMN_WIDTH = 40
export const MAX_COLUMN_WIDTH = 640

export type AccountColumnLayout = {
  version: number
  order: string[]
  widths: Record<string, number>
}

export function isPinnedStartColumn(key: string): boolean {
  return (PINNED_START_COLUMNS as readonly string[]).includes(key)
}

export function isPinnedEndColumn(key: string): boolean {
  return (PINNED_END_COLUMNS as readonly string[]).includes(key)
}

export function isReorderableColumn(key: string): boolean {
  return !isPinnedStartColumn(key) && !isPinnedEndColumn(key)
}

/**
 * Merge saved order with current column keys.
 * Always pins select/name first and actions last; appends newly introduced middle columns.
 */
export function mergeAccountColumnOrder(saved: string[] | null | undefined, allKeys: string[]): string[] {
  const keySet = new Set(allKeys)
  const pinnedStart = PINNED_START_COLUMNS.filter((key) => keySet.has(key))
  const pinnedEnd = PINNED_END_COLUMNS.filter((key) => keySet.has(key))
  const middleDefault = allKeys.filter(
    (key) => !pinnedStart.includes(key as (typeof PINNED_START_COLUMNS)[number]) && !pinnedEnd.includes(key as (typeof PINNED_END_COLUMNS)[number])
  )

  if (!saved || saved.length === 0) {
    return [...pinnedStart, ...middleDefault, ...pinnedEnd]
  }

  const middleSaved = saved.filter((key) => middleDefault.includes(key))
  const missing = middleDefault.filter((key) => !middleSaved.includes(key))
  return [...pinnedStart, ...middleSaved, ...missing, ...pinnedEnd]
}

export function clampColumnWidth(key: string, width: number): number {
  const min = key === 'select' ? MIN_SELECT_COLUMN_WIDTH : MIN_COLUMN_WIDTH
  if (!Number.isFinite(width)) return min
  return Math.min(MAX_COLUMN_WIDTH, Math.max(min, Math.round(width)))
}

export function normalizeAccountColumnWidths(
  raw: Record<string, unknown> | null | undefined,
  allKeys: string[]
): Record<string, number> {
  const out: Record<string, number> = {}
  if (!raw || typeof raw !== 'object') return out
  const keySet = new Set(allKeys)
  for (const [key, value] of Object.entries(raw)) {
    if (!keySet.has(key)) continue
    const num = typeof value === 'number' ? value : Number(value)
    if (!Number.isFinite(num)) continue
    out[key] = clampColumnWidth(key, num)
  }
  return out
}

export function parseAccountColumnLayout(
  raw: string | null,
  allKeys: string[]
): AccountColumnLayout {
  if (!raw) {
    return {
      version: ACCOUNT_COLUMN_LAYOUT_VERSION,
      order: mergeAccountColumnOrder(null, allKeys),
      widths: {}
    }
  }
  try {
    const parsed = JSON.parse(raw) as Partial<AccountColumnLayout>
    const order = mergeAccountColumnOrder(
      Array.isArray(parsed.order) ? parsed.order.filter((k): k is string => typeof k === 'string') : null,
      allKeys
    )
    const widths = normalizeAccountColumnWidths(
      parsed.widths && typeof parsed.widths === 'object' ? (parsed.widths as Record<string, unknown>) : null,
      allKeys
    )
    return {
      version: ACCOUNT_COLUMN_LAYOUT_VERSION,
      order,
      widths
    }
  } catch {
    return {
      version: ACCOUNT_COLUMN_LAYOUT_VERSION,
      order: mergeAccountColumnOrder(null, allKeys),
      widths: {}
    }
  }
}

/** Move a reorderable column one step within the middle segment. */
export function moveAccountColumnOrder(
  order: string[],
  key: string,
  direction: 'up' | 'down'
): string[] {
  if (!isReorderableColumn(key)) return [...order]
  const next = [...order]
  const index = next.indexOf(key)
  if (index < 0) return next

  const targetIndex = direction === 'up' ? index - 1 : index + 1
  if (targetIndex < 0 || targetIndex >= next.length) return next
  if (!isReorderableColumn(next[targetIndex])) return next

  ;[next[index], next[targetIndex]] = [next[targetIndex], next[index]]
  return next
}
