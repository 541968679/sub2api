export type LatencySeverity = 'good' | 'warn' | 'slow' | 'critical'

const classify = (milliseconds: number, thresholds: readonly [number, number, number]): LatencySeverity => {
  if (milliseconds < thresholds[0]) return 'good'
  if (milliseconds < thresholds[1]) return 'warn'
  if (milliseconds < thresholds[2]) return 'slow'
  return 'critical'
}

export const firstTokenSeverity = (milliseconds: number): LatencySeverity =>
  classify(milliseconds, [10_000, 30_000, 60_000])

export const durationSeverity = (milliseconds: number): LatencySeverity =>
  classify(milliseconds, [60_000, 180_000, 300_000])

export const LATENCY_TEXT_CLASSES: Record<LatencySeverity, string> = {
  good: 'text-emerald-600 dark:text-emerald-400',
  warn: 'text-amber-600 dark:text-amber-400',
  slow: 'text-orange-600 dark:text-orange-400',
  critical: 'text-red-600 dark:text-red-400',
}

export const LATENCY_BAR_CLASSES: Record<LatencySeverity, string> = {
  good: 'bg-emerald-500',
  warn: 'bg-amber-400',
  slow: 'bg-orange-500',
  critical: 'bg-red-500',
}

export const LATENCY_BAR_FROM_CLASSES: Record<LatencySeverity, string> = {
  good: 'from-emerald-500',
  warn: 'from-amber-400',
  slow: 'from-orange-500',
  critical: 'from-red-500',
}

export const LATENCY_BAR_TO_CLASSES: Record<LatencySeverity, string> = {
  good: 'to-emerald-500',
  warn: 'to-amber-400',
  slow: 'to-orange-500',
  critical: 'to-red-500',
}

export const migrateLatencyHiddenColumns = (columns: string[]): string[] => {
  const migrated: string[] = []
  for (const key of columns) {
    const nextKey = key === 'first_token' || key === 'duration' ? 'latency' : key
    if (!migrated.includes(nextKey)) migrated.push(nextKey)
  }
  return migrated
}
