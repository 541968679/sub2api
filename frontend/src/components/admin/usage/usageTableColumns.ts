import { migrateLatencyHiddenColumns } from '@/utils/latencyHealth'

export const ALWAYS_VISIBLE_USAGE_COLUMNS = ['user', 'created_at', 'actions'] as const

export const DEFAULT_HIDDEN_USAGE_COLUMNS = ['reasoning_effort', 'user_agent']

export const USAGE_HIDDEN_COLUMNS_KEY = 'usage-hidden-columns'

export type AdminUsageTableColumn = {
  key: string
  label: string
  sortable: boolean
  preserveHeaderCase?: boolean
}

export function buildAdminUsageTableColumns(t: (key: string) => string): AdminUsageTableColumn[] {
  return [
    { key: 'user', label: t('admin.usage.user'), sortable: false },
    { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
    { key: 'account', label: t('admin.usage.account'), sortable: false },
    { key: 'model', label: t('usage.model'), sortable: true },
    { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
    { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
    { key: 'group', label: t('admin.usage.group'), sortable: false },
    { key: 'stream', label: t('usage.type'), sortable: false },
    { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
    { key: 'tokens', label: t('admin.usage.tokenColumn'), sortable: false, preserveHeaderCase: true },
    { key: 'display_tokens', label: t('admin.usage.displayTokenColumn'), sortable: false, preserveHeaderCase: true },
    { key: 'cost', label: t('usage.cost'), sortable: false },
    { key: 'latency', label: t('usage.latency'), sortable: false },
    { key: 'true_first_token', label: t('usage.trueFirstToken'), sortable: false },
    { key: 'created_at', label: t('usage.time'), sortable: true },
    { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
    { key: 'ip_address', label: t('admin.usage.ipAddress'), sortable: false },
    { key: 'actions', label: t('admin.usage.actions'), sortable: false }
  ]
}

export function loadHiddenUsageColumns(): string[] {
  try {
    const saved = localStorage.getItem(USAGE_HIDDEN_COLUMNS_KEY)
    if (saved) {
      return migrateLatencyHiddenColumns(JSON.parse(saved) as string[])
    }
  } catch {
    // fall through to defaults
  }
  return [...DEFAULT_HIDDEN_USAGE_COLUMNS]
}

export function saveHiddenUsageColumns(keys: string[]): void {
  try {
    localStorage.setItem(USAGE_HIDDEN_COLUMNS_KEY, JSON.stringify(keys))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}
