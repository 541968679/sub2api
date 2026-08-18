import type { OpsErrorLog } from '@/api/admin/ops'

export type ErrorLogCaliberBadge = {
  key: string
  labelKey: string
  className: string
}

const chipExcluded =
  'bg-slate-100 text-slate-700 ring-slate-600/20 dark:bg-dark-700 dark:text-slate-300 dark:ring-dark-500/40'
const chipIncluded =
  'bg-sky-50 text-sky-700 ring-sky-600/20 dark:bg-sky-900/30 dark:text-sky-300 dark:ring-sky-500/30'
const chipScheduleOn =
  'bg-amber-50 text-amber-800 ring-amber-600/20 dark:bg-amber-900/30 dark:text-amber-200 dark:ring-amber-500/30'
const chipScheduleOff =
  'bg-gray-100 text-gray-700 ring-gray-600/20 dark:bg-dark-700 dark:text-gray-300 dark:ring-dark-500/40'

export function isRecoveredErrorLog(log: OpsErrorLog): boolean {
  if (log.is_recovered === true) return true
  if (log.is_recovered === false) return false
  const msg = String(log.message || '')
  return /^Recovered\b/i.test(msg) && Number(log.status_code || 0) < 400
}

export function errorLogCaliberBadges(log: OpsErrorLog): ErrorLogCaliberBadge[] {
  if (
    log.counted_in_user_error_rate == null &&
    log.counted_in_account_compare_rate == null &&
    log.counted_in_account_schedule_rate == null
  ) {
    return []
  }
  const badges: ErrorLogCaliberBadge[] = []
  if (log.counted_in_user_error_rate === false) {
    badges.push({
      key: 'user-excluded',
      labelKey: 'admin.ops.errorLog.caliberUserExcluded',
      className: chipExcluded
    })
  } else if (log.counted_in_account_compare_rate === false) {
    badges.push({
      key: 'user-included',
      labelKey: 'admin.ops.errorLog.caliberUserIncluded',
      className: chipIncluded
    })
  }
  if (log.counted_in_account_compare_rate === true) {
    badges.push({
      key: 'compare-included',
      labelKey: 'admin.ops.errorLog.caliberCompareIncluded',
      className: chipIncluded
    })
  } else if (log.counted_in_account_compare_rate === false) {
    badges.push({
      key: 'compare-excluded',
      labelKey: 'admin.ops.errorLog.caliberCompareExcluded',
      className: chipExcluded
    })
  }
  if (log.counted_in_account_schedule_rate === true) {
    badges.push({
      key: 'schedule-included',
      labelKey: 'admin.ops.errorLog.caliberScheduleIncluded',
      className: chipScheduleOn
    })
  } else if (log.counted_in_account_schedule_rate === false) {
    badges.push({
      key: 'schedule-excluded',
      labelKey: 'admin.ops.errorLog.caliberScheduleExcluded',
      className: chipScheduleOff
    })
  }
  return badges
}
