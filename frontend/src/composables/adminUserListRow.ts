import type { AdminGroup, AdminUser, UserStatus } from '@/types'
import type { Column } from '@/components/common/types'
import type { SmartScheduleSummary } from '@/api/admin/users'

export function resolveAdminUserGroups(user: AdminUser, groups: AdminGroup[]) {
  const exclusive: AdminGroup[] = []
  const publicGroups: AdminGroup[] = []
  for (const group of groups) {
    if (group.status !== 'active' || group.subscription_type !== 'standard') continue
    if (group.is_exclusive) {
      if (user.allowed_groups?.includes(group.id)) exclusive.push(group)
    } else {
      publicGroups.push(group)
    }
  }
  return { exclusive, publicGroups }
}

export function getAdminUserStatusLabel(
  status: UserStatus | string,
  t: (key: string) => string
) {
  switch (status) {
    case 'active':
      return t('common.active')
    case 'pending_approval':
      return t('admin.users.pendingApproval')
    case 'disabled':
      return t('admin.users.disabled')
    default:
      return status || t('common.unknown')
  }
}

export function getAdminUserStatusDotClass(status: UserStatus | string) {
  switch (status) {
    case 'active':
      return 'bg-green-500'
    case 'pending_approval':
      return 'bg-amber-500'
    default:
      return 'bg-red-500'
  }
}

export function getAdminUserStatusTextClass(status: UserStatus | string) {
  switch (status) {
    case 'active':
      return 'text-green-700 dark:text-green-300'
    case 'pending_approval':
      return 'text-amber-700 dark:text-amber-300'
    default:
      return 'text-gray-700 dark:text-gray-300'
  }
}

export function getSubscriptionDaysRemaining(expiresAt: string): number {
  const diffMs = new Date(expiresAt).getTime() - Date.now()
  return Math.ceil(diffMs / (1000 * 60 * 60 * 24))
}

export function smartScheduleSummaryFromDrafts(
  platforms: string[],
  drafts: Record<string, { enabled?: boolean; accounts?: unknown[] } | undefined>
): SmartScheduleSummary {
  const enabled_platforms: string[] = []
  const pool_counts: Record<string, number> = {}
  for (const platform of platforms) {
    const draft = drafts[platform]
    const count = draft?.accounts?.length ?? 0
    pool_counts[platform] = count
    if (draft?.enabled && count > 0) enabled_platforms.push(platform)
  }
  return { enabled_platforms, pool_counts }
}

export function buildAdminUserListRowColumns(t: (key: string) => string): Column[] {
  return [
    { key: 'email', label: t('admin.users.columns.user'), sortable: false },
    { key: 'id', label: t('admin.users.columns.id'), sortable: false },
    { key: 'username', label: t('admin.users.columns.username'), sortable: false },
    { key: 'role', label: t('admin.users.columns.role'), sortable: false },
    { key: 'groups', label: t('admin.users.columns.groups'), sortable: false },
    { key: 'subscriptions', label: t('admin.users.columns.subscriptions'), sortable: false },
    { key: 'balance', label: t('admin.users.columns.balance'), sortable: false },
    { key: 'usage', label: t('admin.users.columns.usage'), sortable: false },
    { key: 'concurrency', label: t('admin.users.columns.concurrency'), sortable: false },
    { key: 'smart_schedule', label: t('admin.users.columns.smartSchedule'), sortable: false },
    { key: 'quality_ttft', label: t('admin.users.columns.qualityTtft'), sortable: false },
    { key: 'quality_success_rate', label: t('admin.users.columns.qualitySuccessRate'), sortable: false },
    { key: 'status', label: t('admin.users.columns.status'), sortable: false },
    { key: 'last_active_at', label: t('admin.users.columns.lastActive'), sortable: false },
    { key: 'last_used_at', label: t('admin.users.columns.lastUsed'), sortable: false },
    { key: 'created_at', label: t('admin.users.columns.created'), sortable: false }
  ]
}
