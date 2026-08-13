<template>
  <BaseDialog
    :show="show"
    :title="dialogTitle"
    width="full"
    :close-on-click-outside="!nestedOverlayOpen"
    :close-on-escape="!nestedOverlayOpen"
    data-testid="usage-error-inspect-dialog"
    @close="$emit('close')"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex gap-1 border-b border-gray-200 dark:border-dark-700">
          <button
            v-for="tab in inspectTabs"
            :key="tab.key"
            type="button"
            data-testid="inspect-detail-tab"
            class="border-b-2 px-4 py-2 text-sm font-medium"
            :class="activeTab === tab.key ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500'"
            @click="switchTab(tab.key)"
          >
            {{ tab.label }}
          </button>
        </div>
        <button
          type="button"
          data-testid="inspect-view-full"
          class="btn btn-secondary"
          @click="openFullPage"
        >
          {{ t('admin.usage.inspectViewFull') }}
        </button>
      </div>

      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.timeRange') }}:</span>
          <DateRangePicker
            v-model:start-date="startDate"
            v-model:end-date="endDate"
            @change="onDateRangeChange"
          />
        </div>
      </div>

      <template v-if="activeTab === 'usage'">
        <UsageFilters
          v-model="filters"
          :start-date="startDate"
          :end-date="endDate"
          :exporting="false"
          :show-export="false"
          :show-cleanup="false"
          :locked-user-id="lockedUserId"
          :locked-account-id="lockedAccountId"
          :locked-user-label="lockedUserLabel"
          :locked-account-label="lockedAccountLabel"
          @change="applyUsageFilters"
          @refresh="loadLogs"
          @reset="resetFilters"
        >
          <template #after-reset>
            <div class="relative" ref="columnDropdownRef">
              <button
                type="button"
                class="btn btn-secondary px-2 md:px-3"
                :title="t('admin.users.columnSettings')"
                @click="showColumnDropdown = !showColumnDropdown"
              >
                <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
              </button>
              <div
                v-if="showColumnDropdown"
                class="absolute right-0 top-full z-50 mt-1 max-h-80 w-48 overflow-y-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
              >
                <button
                  v-for="col in toggleableColumns"
                  :key="col.key"
                  type="button"
                  class="flex w-full items-center justify-between px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
                  @click="toggleColumn(col.key)"
                >
                  <span>{{ col.label }}</span>
                  <span v-if="isColumnVisible(col.key)" class="text-primary-500">✓</span>
                </button>
              </div>
            </div>
          </template>
        </UsageFilters>
        <UsageTable
          :data="usageLogs"
          :loading="loading"
          :columns="visibleColumns"
          :server-side-sort="true"
          :default-sort-key="'created_at'"
          :default-sort-order="'desc'"
          @sort="handleSort"
          @userClick="handleUserClick"
          @userViewClick="handleUserViewClick"
        />
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>

      <template v-else>
        <ErrorRequestFilters
          v-model="errorFilters"
          :start-date="startDate"
          :end-date="endDate"
          :locked-user-id="lockedUserId"
          :locked-account-id="lockedAccountId"
          :locked-user-label="lockedUserLabel"
          :locked-account-label="lockedAccountLabel"
          @change="onErrorFiltersChange"
          @refresh="loadErrorLogs"
          @reset="resetErrorFilters"
        />
        <OpsErrorLogTable
          v-if="errorsMounted"
          :rows="errorLogs"
          :total="errorTotal"
          :loading="errorLoading"
          :page="errorPage"
          :page-size="errorPageSize"
          @openErrorDetail="openErrorDetail"
          @update:page="handleErrorPageChange"
          @update:pageSize="handleErrorPageSizeChange"
        />
      </template>
    </div>
  </BaseDialog>

  <UserBalanceHistoryModal
    :show="showBalanceHistoryModal"
    :user="balanceHistoryUser"
    :hide-actions="true"
    :z-index="60"
    @close="showBalanceHistoryModal = false; balanceHistoryUser = null"
  />
  <UserViewCompareDrawer
    :log-id="userViewLogId"
    :open="userViewOpen"
    :z-index="60"
    @close="closeUserViewDrawer"
  />
  <OpsErrorDetailModal
    v-model:show="showErrorDetailModal"
    :error-id="selectedErrorId"
    :error-type="'request'"
    :z-index="60"
  />
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { AdminUsageQueryParams } from '@/api/admin/usage'
import { opsAPI, type OpsErrorLog } from '@/api/admin/ops'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { requestTypeToLegacyStream } from '@/utils/usageRequestType'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Pagination from '@/components/common/Pagination.vue'
import UsageFilters from '@/components/admin/usage/UsageFilters.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'
import ErrorRequestFilters, {
  type ErrorRequestFilterState
} from '@/components/admin/usage/ErrorRequestFilters.vue'
import OpsErrorLogTable from '@/views/admin/ops/components/OpsErrorLogTable.vue'
import OpsErrorDetailModal from '@/views/admin/ops/components/OpsErrorDetailModal.vue'
import UserViewCompareDrawer from '@/components/admin/usage/UserViewCompareDrawer.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import {
  ALWAYS_VISIBLE_USAGE_COLUMNS,
  buildAdminUsageTableColumns,
  loadHiddenUsageColumns,
  saveHiddenUsageColumns
} from '@/components/admin/usage/usageTableColumns'
import type { AdminUsageLog, AdminUser } from '@/types'

export type UsageErrorInspectScope = 'account' | 'user'
export type UsageErrorInspectTab = 'usage' | 'errors'

const props = withDefaults(defineProps<{
  show: boolean
  scope: UsageErrorInspectScope
  subjectId: number | null
  subjectLabel?: string
  initialTab?: UsageErrorInspectTab
}>(), {
  initialTab: 'usage'
})

defineEmits<{ close: [] }>()

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const activeTab = ref<UsageErrorInspectTab>('usage')
const inspectTabs = computed(() => [
  { key: 'usage' as const, label: t('usage.tabs.usage') },
  { key: 'errors' as const, label: t('usage.tabs.errors') }
])

const dialogTitle = computed(() => {
  const label = props.subjectLabel?.trim()
  const id = props.subjectId
  if (label && id) return `${label} (#${id})`
  if (id) return `#${id}`
  return t('admin.usage.inspectTitle')
})

const lockedUserId = computed(() => (props.scope === 'user' ? props.subjectId ?? undefined : undefined))
const lockedAccountId = computed(() => (props.scope === 'account' ? props.subjectId ?? undefined : undefined))
const lockedUserLabel = computed(() => (props.scope === 'user' ? props.subjectLabel : undefined))
const lockedAccountLabel = computed(() => (props.scope === 'account' ? props.subjectLabel : undefined))

const formatLD = (d: Date) => {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLD(start), end: formatLD(end) }
}

const emptyErrorFilters = (): ErrorRequestFilterState => ({
  user_id: undefined,
  api_key_id: undefined,
  model: null,
  account_id: undefined,
  group_id: null,
  platform: '',
  bridge: 'all',
  upstream_model: '',
  q: '',
  status_codes: []
})

const applyLockedIds = <T extends { user_id?: number; account_id?: number }>(base: T): T => {
  if (props.scope === 'user' && props.subjectId) {
    return { ...base, user_id: props.subjectId }
  }
  if (props.scope === 'account' && props.subjectId) {
    return { ...base, account_id: props.subjectId }
  }
  return base
}

const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const filters = ref<AdminUsageQueryParams>(applyLockedIds({
  user_id: undefined,
  model: undefined,
  group_id: undefined,
  request_type: undefined,
  billing_type: null,
  start_date: startDate.value,
  end_date: endDate.value
}))
const errorFilters = ref<ErrorRequestFilterState>(applyLockedIds(emptyErrorFilters()))

const usageLogs = ref<AdminUsageLog[]>([])
const loading = ref(false)
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})
let abortController: AbortController | null = null

const errorsMounted = ref(false)
const errorLogs = ref<OpsErrorLog[]>([])
const errorTotal = ref(0)
const errorLoading = ref(false)
const errorPage = ref(1)
const errorPageSize = ref(getPersistedPageSize())
const showErrorDetailModal = ref(false)
const selectedErrorId = ref<number | null>(null)

const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)
const userViewLogId = ref<number | null>(null)
const userViewOpen = ref(false)
const nestedOverlayOpen = computed(
  () => showBalanceHistoryModal.value || userViewOpen.value || showErrorDetailModal.value
)

const isCanceledRequest = (error: unknown, signal?: AbortSignal) => {
  if (signal?.aborted) return true
  const err = error as { name?: string; code?: string } | null
  return err?.name === 'AbortError' || err?.name === 'CanceledError' || err?.code === 'ERR_CANCELED'
}

const allColumns = computed(() => buildAdminUsageTableColumns(t))
const hiddenColumns = reactive<Set<string>>(new Set(loadHiddenUsageColumns()))
const ALWAYS_VISIBLE: readonly string[] = ALWAYS_VISIBLE_USAGE_COLUMNS
const toggleableColumns = computed(() =>
  allColumns.value.filter((col) => !ALWAYS_VISIBLE.includes(col.key))
)
const visibleColumns = computed(() =>
  allColumns.value.filter((col) =>
    ALWAYS_VISIBLE.includes(col.key) || !hiddenColumns.has(col.key)
  )
)
const isColumnVisible = (key: string) => !hiddenColumns.has(key)
const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) hiddenColumns.delete(key)
  else hiddenColumns.add(key)
  saveHiddenUsageColumns([...hiddenColumns])
}
const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)

const toDayStartISO = (d: string) => new Date(`${d}T00:00:00`).toISOString()
const toDayEndISO = (d: string) => new Date(`${d}T23:59:59.999`).toISOString()

const buildUsageListParams = (page: number, pageSize: number): AdminUsageQueryParams => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return applyLockedIds({
    page,
    page_size: pageSize,
    exact_total: false,
    ...filters.value,
    stream: legacyStream === null ? undefined : legacyStream,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order,
    start_date: startDate.value,
    end_date: endDate.value
  })
}

const loadLogs = async () => {
  abortController?.abort()
  const c = new AbortController()
  abortController = c
  loading.value = true
  try {
    const res = await adminAPI.usage.list(
      buildUsageListParams(pagination.page, pagination.page_size),
      { signal: c.signal }
    )
    if (!c.signal.aborted) {
      usageLogs.value = res.items
      pagination.total = res.total
    }
  } catch (error: unknown) {
    if (!isCanceledRequest(error, c.signal)) {
      console.error('Failed to load usage logs:', error)
      appStore.showError(t('usage.failedToLoad'))
    }
  } finally {
    if (abortController === c) loading.value = false
  }
}

const buildErrorQueryParams = (includePagination: boolean): Record<string, any> => {
  const f = applyLockedIds({ ...errorFilters.value })
  const params: Record<string, any> = {
    start_time: toDayStartISO(startDate.value),
    end_time: toDayEndISO(endDate.value),
    view: f.status_codes.length > 0 ? 'all' : 'errors'
  }
  if (includePagination) {
    params.page = errorPage.value
    params.page_size = errorPageSize.value
  }
  if (f.user_id) params.user_id = f.user_id
  if (f.api_key_id) params.api_key_id = f.api_key_id
  if (f.model) params.model = f.model
  if (f.account_id) params.account_id = f.account_id
  if (f.group_id) params.group_id = f.group_id
  if (f.platform) params.platform = f.platform
  if (f.bridge && f.bridge !== 'all') params.bridge = f.bridge
  if (f.upstream_model.trim()) params.upstream_model = f.upstream_model.trim()
  if (f.q.trim()) params.q = f.q.trim()
  if (f.status_codes.length > 0) params.status_codes = f.status_codes.join(',')
  return params
}

const loadErrorLogs = async () => {
  errorLoading.value = true
  try {
    const res = await opsAPI.listErrorLogs(buildErrorQueryParams(true))
    errorLogs.value = res.items || []
    errorTotal.value = res.total || 0
  } catch (error) {
    console.error('Failed to load error logs:', error)
    errorLogs.value = []
    errorTotal.value = 0
    appStore.showError(t('usage.errors.failedToLoad'))
  } finally {
    errorLoading.value = false
  }
}

const seedFromSubject = () => {
  closeNestedOverlays()
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = applyLockedIds({
    user_id: undefined,
    model: undefined,
    group_id: undefined,
    request_type: undefined,
    billing_type: null,
    billing_mode: undefined,
    start_date: range.start,
    end_date: range.end
  })
  errorFilters.value = applyLockedIds(emptyErrorFilters())
  pagination.page = 1
  errorPage.value = 1
  errorsMounted.value = props.initialTab === 'errors'
  activeTab.value = props.initialTab
}

const applyUsageFilters = () => {
  pagination.page = 1
  filters.value = applyLockedIds({
    ...filters.value,
    start_date: startDate.value,
    end_date: endDate.value
  })
  void loadLogs()
}

const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = applyLockedIds<AdminUsageQueryParams>({
    start_date: range.start,
    end_date: range.end,
    request_type: undefined,
    billing_type: null,
    billing_mode: undefined
  })
  applyUsageFilters()
}

const onDateRangeChange = (range: { startDate: string; endDate: string }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  if (activeTab.value === 'errors') {
    errorPage.value = 1
    void loadErrorLogs()
    return
  }
  applyUsageFilters()
}

const handlePageChange = (p: number) => { pagination.page = p; void loadLogs() }
const handlePageSizeChange = (s: number) => { pagination.page_size = s; pagination.page = 1; void loadLogs() }
const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadLogs()
}

const onErrorFiltersChange = () => {
  errorFilters.value = applyLockedIds({ ...errorFilters.value })
  errorPage.value = 1
  void loadErrorLogs()
}
const resetErrorFilters = () => {
  errorFilters.value = applyLockedIds(emptyErrorFilters())
  errorPage.value = 1
  void loadErrorLogs()
}
const handleErrorPageChange = (p: number) => { errorPage.value = p; void loadErrorLogs() }
const handleErrorPageSizeChange = (s: number) => { errorPageSize.value = s; errorPage.value = 1; void loadErrorLogs() }
const openErrorDetail = (id: number) => { selectedErrorId.value = id; showErrorDetailModal.value = true }

const switchTab = (tab: UsageErrorInspectTab) => {
  activeTab.value = tab
  if (tab === 'errors') {
    errorsMounted.value = true
    void loadErrorLogs()
  }
}

const handleUserClick = async (userId: number) => {
  try {
    const user = await adminAPI.users.getById(userId)
    balanceHistoryUser.value = user
    showBalanceHistoryModal.value = true
  } catch {
    appStore.showError(t('admin.usage.failedToLoadUser'))
  }
}
const handleUserViewClick = (logId: number) => {
  userViewLogId.value = logId
  userViewOpen.value = true
}
const closeUserViewDrawer = () => {
  userViewOpen.value = false
  userViewLogId.value = null
}

const closeNestedOverlays = () => {
  showBalanceHistoryModal.value = false
  balanceHistoryUser.value = null
  closeUserViewDrawer()
  showErrorDetailModal.value = false
  selectedErrorId.value = null
}

const openFullPage = () => {
  if (!props.subjectId) return
  const query: Record<string, string> = {}
  if (props.scope === 'account') {
    query.account_id = String(props.subjectId)
    if (props.subjectLabel) query.account_name = props.subjectLabel
  } else {
    query.user_id = String(props.subjectId)
    if (props.subjectLabel) query.user_email = props.subjectLabel
  }
  if (activeTab.value === 'errors') query.tab = 'errors'
  const resolved = router.resolve({ path: '/admin/usage', query })
  window.open(resolved.href, '_blank', 'noopener,noreferrer')
}

watch(
  () => [props.show, props.subjectId, props.scope, props.initialTab] as const,
  ([show]) => {
    if (!show) {
      abortController?.abort()
      closeNestedOverlays()
      return
    }
    if (!props.subjectId) return
    seedFromSubject()
    if (activeTab.value === 'errors') {
      void loadErrorLogs()
    } else {
      void loadLogs()
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  abortController?.abort()
})
</script>
