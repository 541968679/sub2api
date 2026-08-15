import type { Account, AccountScheduleUser } from '@/types'
import { buildOpenAIUsageRefreshKey } from '@/utils/accountUsageRefresh'

const normalizeRefreshValue = (value: unknown): string => {
  if (value == null) return ''
  return String(value)
}

/** Stable key for schedule-user chips so list refresh can see quality runtime changes. */
export function scheduleUsersRefreshKey(users: AccountScheduleUser[] | undefined): string {
  return (users ?? [])
    .slice()
    .sort((left, right) => left.id - right.id)
    .map((user) => [
      user.id,
      user.allow ? 1 : 0,
      user.deny ? 1 : 0,
      user.max_concurrency ?? '',
      user.quality_max_p50_ttft_ms ?? '',
      user.quality_min_success_rate ?? '',
      user.quality_min_success_samples ?? '',
      user.quality_min_ttft_samples ?? '',
      user.quality_condition ?? '',
      user.quality_blocked ? 1 : 0,
      user.quality_resumed_until ?? '',
      user.quality_window_until ?? ''
    ].map(normalizeRefreshValue).join(':'))
    .join('|')
}

/** True when auto-refresh must replace the in-memory row instead of keeping the stale object. */
export function shouldReplaceAccountListRow(current: Account, next: Account): boolean {
  return (
    current.updated_at !== next.updated_at ||
    current.current_concurrency !== next.current_concurrency ||
    current.current_window_cost !== next.current_window_cost ||
    current.active_sessions !== next.active_sessions ||
    current.schedulable !== next.schedulable ||
    current.status !== next.status ||
    current.rate_limit_reset_at !== next.rate_limit_reset_at ||
    current.overload_until !== next.overload_until ||
    current.temp_unschedulable_until !== next.temp_unschedulable_until ||
    buildOpenAIUsageRefreshKey(current) !== buildOpenAIUsageRefreshKey(next) ||
    scheduleUsersRefreshKey(current.schedule_users) !== scheduleUsersRefreshKey(next.schedule_users)
  )
}
