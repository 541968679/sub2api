<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.redeem.searchCodes')"
              class="input"
              @input="handleSearch"
            />
          </div>
          <Select
            v-model="filters.type"
            :options="filterTypeOptions"
            class="w-36"
            @change="loadBatches"
          />
          <Select
            v-model="filters.status"
            :options="filterStatusOptions"
            class="w-36"
            @change="loadBatches"
          />

          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              @click="loadBatches"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="handleExportCodes" class="btn btn-secondary">
              {{ t('admin.redeem.exportCsv') }}
            </button>
            <button @click="openGenerateDialog" class="btn btn-primary">
              {{ t('admin.redeem.generateCodes') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="batchColumns"
          :data="batches"
          :loading="loading"
          :server-side-sort="false"
          default-sort-key="created_at"
          default-sort-order="desc"
        >
          <template #cell-type="{ value }">
            <span
              :class="[
                'badge',
                value === 'balance'
                  ? 'badge-success'
                  : value === 'subscription'
                    ? 'badge-warning'
                    : 'badge-primary'
              ]"
            >
              {{ t('admin.redeem.types.' + value) }}
            </span>
          </template>

          <template #cell-value="{ value, row }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">
              <template v-if="row.type === 'balance'">${{ Number(value).toFixed(2) }}</template>
              <template v-else-if="row.type === 'subscription'">
                {{ row.validity_days || 30 }} {{ t('admin.redeem.days') }}
                <span v-if="row.group_name" class="ml-1 text-xs text-gray-500 dark:text-gray-400">
                  ({{ row.group_name }})
                </span>
              </template>
              <template v-else-if="row.type === 'invitation'">{{ t('admin.redeem.invitation') }}</template>
              <template v-else>{{ value }}</template>
            </span>
          </template>

          <template #cell-counts="{ row }">
            <div class="flex flex-wrap gap-1.5 text-xs">
              <span class="rounded-full bg-gray-100 px-2 py-0.5 text-gray-700 dark:bg-dark-700 dark:text-gray-200">
                {{ t('admin.redeem.totalShort', { n: row.total_count }) }}
              </span>
              <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                {{ t('admin.redeem.unusedShort', { n: row.unused_count }) }}
              </span>
              <span class="rounded-full bg-slate-100 px-2 py-0.5 text-slate-600 dark:bg-dark-600 dark:text-gray-300">
                {{ t('admin.redeem.usedShort', { n: row.used_count }) }}
              </span>
              <span
                v-if="row.expired_count > 0"
                class="rounded-full bg-amber-50 px-2 py-0.5 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
              >
                {{ t('admin.redeem.expiredShort', { n: row.expired_count }) }}
              </span>
            </div>
          </template>

          <template #cell-batch_redeem_limit_per_user="{ value }">
            <span class="text-sm text-gray-600 dark:text-gray-300">
              {{ value ? t('common.yes') : t('common.no') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex flex-wrap items-center gap-1">
              <button
                class="btn btn-secondary px-2 py-1 text-xs"
                @click="openBatchDetail(row)"
              >
                {{ t('admin.redeem.detail') }}
              </button>
              <button
                v-if="row.unused_count > 0"
                class="btn btn-secondary px-2 py-1 text-xs"
                @click="copyBatchUnused(row)"
              >
                {{ t('admin.redeem.copyUnused') }}
              </button>
              <button
                v-if="row.unused_count > 0"
                class="btn btn-danger px-2 py-1 text-xs"
                @click="confirmDeleteBatchUnused(row)"
              >
                {{ t('admin.redeem.deleteUnusedInBatch') }}
              </button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Batch detail drawer -->
    <Teleport to="body">
      <div v-if="showDetail" class="fixed inset-0 z-50 flex justify-end">
        <div class="fixed inset-0 bg-black/40" @click="closeBatchDetail"></div>
        <div
          class="relative z-10 flex h-full w-full max-w-xl flex-col bg-white shadow-2xl dark:bg-dark-800"
        >
          <div class="flex items-start justify-between border-b border-gray-200 px-5 py-4 dark:border-dark-600">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.redeem.batchDetailTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ detailBatchSummary }}
              </p>
            </div>
            <button
              class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 dark:hover:bg-dark-700"
              @click="closeBatchDetail"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <div class="flex flex-wrap gap-2 border-b border-gray-100 px-5 py-3 dark:border-dark-700">
            <button
              class="btn btn-secondary text-sm"
              :disabled="detailUnusedCodes.length === 0"
              @click="copyDetailUnused"
            >
              {{ t('admin.redeem.copyUnused') }}
            </button>
            <button
              class="btn btn-danger text-sm"
              :disabled="detailUnusedCodes.length === 0 || detailDeleting"
              @click="deleteDetailUnused"
            >
              {{ t('admin.redeem.deleteUnusedInBatch') }}
            </button>
          </div>

          <div class="flex-1 overflow-y-auto p-5">
            <div v-if="detailLoading" class="py-12 text-center text-gray-500">
              {{ t('common.loading') }}
            </div>
            <div v-else-if="detailCodes.length === 0" class="py-12 text-center text-gray-500">
              {{ t('admin.redeem.batchEmpty') }}
            </div>
            <ul v-else class="space-y-2">
              <li
                v-for="code in detailCodes"
                :key="code.id"
                class="flex items-center justify-between gap-3 rounded-lg border border-gray-100 px-3 py-2.5 dark:border-dark-600"
              >
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <code class="truncate font-mono text-sm text-gray-900 dark:text-gray-100">{{
                      code.code
                    }}</code>
                    <button
                      class="shrink-0 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                      @click="copyToClipboard(code.code)"
                    >
                      <Icon name="copy" size="sm" />
                    </button>
                  </div>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.redeem.status.' + code.status) }}
                    <template v-if="code.user?.email"> · {{ code.user.email }}</template>
                    <template v-else-if="code.used_by">
                      · {{ t('admin.redeem.userPrefix', { id: code.used_by }) }}
                    </template>
                    <template v-if="code.used_at"> · {{ formatDateTime(code.used_at) }}</template>
                  </p>
                </div>
                <button
                  v-if="code.status === 'unused'"
                  class="shrink-0 text-xs text-red-600 hover:underline dark:text-red-400"
                  @click="deleteSingleCode(code)"
                >
                  {{ t('common.delete') }}
                </button>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Generate: left config + right result -->
    <Teleport to="body">
      <div v-if="showGenerateDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="showGenerateDialog = false"></div>
        <div
          class="relative z-10 flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-xl bg-white shadow-xl dark:bg-dark-800"
        >
          <div
            class="flex items-center justify-between border-b border-gray-200 px-6 py-4 dark:border-dark-600"
          >
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.redeem.generateCodesTitle') }}
            </h2>
            <button
              class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 dark:hover:bg-dark-700"
              @click="showGenerateDialog = false"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <div class="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-2">
            <!-- Left: form -->
            <div class="overflow-y-auto border-b border-gray-200 p-6 dark:border-dark-600 lg:border-b-0 lg:border-r">
              <form class="space-y-4" @submit.prevent="handleGenerateCodes">
                <div>
                  <label class="input-label">{{ t('admin.redeem.codeType') }}</label>
                  <Select v-model="generateForm.type" :options="typeOptions" />
                </div>
                <div v-if="generateForm.type !== 'subscription' && generateForm.type !== 'invitation'">
                  <label class="input-label">
                    {{
                      generateForm.type === 'balance'
                        ? t('admin.redeem.amount')
                        : t('admin.redeem.columns.value')
                    }}
                  </label>
                  <input
                    v-model.number="generateForm.value"
                    type="number"
                    :step="generateForm.type === 'balance' ? '0.01' : '1'"
                    :min="generateForm.type === 'balance' ? '0.01' : '1'"
                    required
                    class="input"
                  />
                </div>
                <div
                  v-if="generateForm.type === 'invitation'"
                  class="rounded-lg bg-blue-50 p-3 dark:bg-blue-900/20"
                >
                  <p class="text-sm text-blue-700 dark:text-blue-300">
                    {{ t('admin.redeem.invitationHint') }}
                  </p>
                </div>
                <template v-if="generateForm.type === 'subscription'">
                  <div>
                    <label class="input-label">{{ t('admin.redeem.selectGroup') }}</label>
                    <Select
                      v-model="generateForm.group_id"
                      :options="subscriptionGroupOptions"
                      :placeholder="t('admin.redeem.selectGroupPlaceholder')"
                    >
                      <template #selected="{ option }">
                        <GroupBadge
                          v-if="option"
                          :name="(option as unknown as GroupOption).label"
                          :platform="(option as unknown as GroupOption).platform"
                          :subscription-type="(option as unknown as GroupOption).subscriptionType"
                          :rate-multiplier="(option as unknown as GroupOption).rate"
                        />
                        <span v-else class="text-gray-400">{{
                          t('admin.redeem.selectGroupPlaceholder')
                        }}</span>
                      </template>
                      <template #option="{ option, selected }">
                        <GroupOptionItem
                          :name="(option as unknown as GroupOption).label"
                          :platform="(option as unknown as GroupOption).platform"
                          :subscription-type="(option as unknown as GroupOption).subscriptionType"
                          :rate-multiplier="(option as unknown as GroupOption).rate"
                          :description="(option as unknown as GroupOption).description"
                          :selected="selected"
                        />
                      </template>
                    </Select>
                  </div>
                  <div>
                    <label class="input-label">{{ t('admin.redeem.validityDays') }}</label>
                    <input
                      v-model.number="generateForm.validity_days"
                      type="number"
                      min="1"
                      max="365"
                      required
                      class="input"
                    />
                  </div>
                </template>
                <div>
                  <label class="input-label">{{ t('admin.redeem.codeExpiresAt') }}</label>
                  <input v-model="generateForm.expires_at" type="datetime-local" class="input" />
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.redeem.codeExpiresAtHint') }}
                  </p>
                </div>
                <label
                  class="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600"
                >
                  <input
                    v-model="generateForm.batch_redeem_limit_per_user"
                    type="checkbox"
                    class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600"
                  />
                  <span>
                    <span class="block text-sm font-medium text-gray-900 dark:text-white">
                      {{ t('admin.redeem.batchLimitPerUser') }}
                    </span>
                    <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.redeem.batchLimitPerUserHint') }}
                    </span>
                  </span>
                </label>
                <div>
                  <label class="input-label">{{ t('admin.redeem.count') }}</label>
                  <input
                    v-model.number="generateForm.count"
                    type="number"
                    min="1"
                    max="100"
                    required
                    class="input"
                  />
                </div>
                <button type="submit" :disabled="generating" class="btn btn-primary w-full">
                  {{ generating ? t('admin.redeem.generating') : t('admin.redeem.generate') }}
                </button>
              </form>
            </div>

            <!-- Right: latest generated codes -->
            <div class="flex min-h-[280px] flex-col overflow-hidden p-6">
              <div class="mb-3 flex items-center justify-between gap-2">
                <div>
                  <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('admin.redeem.generatedPanelTitle') }}
                  </h3>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {{
                      generatedCodes.length
                        ? t('admin.redeem.codesCreated', { count: generatedCodes.length })
                        : t('admin.redeem.generatedPanelEmpty')
                    }}
                  </p>
                </div>
                <div class="flex gap-2">
                  <button
                    type="button"
                    class="btn btn-secondary text-sm"
                    :disabled="!generatedCodes.length"
                    @click="copyGeneratedCodes"
                  >
                    {{ copiedAll ? t('admin.redeem.copied') : t('admin.redeem.copyAll') }}
                  </button>
                  <button
                    type="button"
                    class="btn btn-primary text-sm"
                    :disabled="!generatedCodes.length"
                    @click="downloadGeneratedCodes"
                  >
                    {{ t('admin.redeem.download') }}
                  </button>
                </div>
              </div>
              <textarea
                readonly
                :value="generatedCodesText"
                class="min-h-0 flex-1 w-full resize-none rounded-lg border border-gray-200 bg-gray-50 p-3 font-mono text-sm text-gray-800 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200"
                :placeholder="t('admin.redeem.generatedPanelEmpty')"
              ></textarea>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <ConfirmDialog
      :show="showDeleteBatchDialog"
      :title="t('admin.redeem.deleteUnusedInBatch')"
      :message="t('admin.redeem.deleteUnusedInBatchConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="executeDeleteBatchUnused"
      @cancel="showDeleteBatchDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import type { RedeemCode, RedeemCodeBatch, RedeemCodeType, Group, GroupPlatform, SubscriptionType } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

interface GroupOption {
  value: number
  label: string
  description: string | null
  platform: GroupPlatform
  subscriptionType: SubscriptionType
  rate: number
}

const showGenerateDialog = ref(false)
const generatedCodes = ref<RedeemCode[]>([])
const subscriptionGroups = ref<Group[]>([])
const copiedAll = ref(false)

const batches = ref<RedeemCodeBatch[]>([])
const loading = ref(false)
const generating = ref(false)
const searchQuery = ref('')
const filters = reactive({ type: '', status: '' })
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const showDetail = ref(false)
const detailBatch = ref<RedeemCodeBatch | null>(null)
const detailCodes = ref<RedeemCode[]>([])
const detailLoading = ref(false)
const detailDeleting = ref(false)

const showDeleteBatchDialog = ref(false)
const pendingDeleteBatch = ref<RedeemCodeBatch | null>(null)

const generateForm = reactive({
  type: 'balance' as RedeemCodeType,
  value: 10,
  count: 1,
  group_id: null as number | null,
  validity_days: 30,
  expires_at: '',
  batch_redeem_limit_per_user: false
})

watch(
  () => generateForm.type,
  (newType) => {
    if (newType === 'invitation') {
      generateForm.value = 0
    } else if (generateForm.value === 0) {
      generateForm.value = 10
    }
  }
)

const subscriptionGroupOptions = computed(() =>
  subscriptionGroups.value
    .filter((g) => g.subscription_type === 'subscription')
    .map((g) => ({
      value: g.id,
      label: g.name,
      description: g.description,
      platform: g.platform,
      subscriptionType: g.subscription_type,
      rate: g.rate_multiplier
    }))
)

const generatedCodesText = computed(() => generatedCodes.value.map((c) => c.code).join('\n'))

const detailUnusedCodes = computed(() => detailCodes.value.filter((c) => c.status === 'unused'))

const detailBatchSummary = computed(() => {
  const b = detailBatch.value
  if (!b) return ''
  return t('admin.redeem.batchDetailSummary', {
    total: b.total_count,
    unused: b.unused_count,
    used: b.used_count
  })
})

const batchColumns = computed<Column[]>(() => [
  { key: 'type', label: t('admin.redeem.columns.type') },
  { key: 'value', label: t('admin.redeem.columns.value') },
  { key: 'counts', label: t('admin.redeem.columns.counts') },
  { key: 'batch_redeem_limit_per_user', label: t('admin.redeem.columns.limitPerUser') },
  { key: 'created_at', label: t('admin.redeem.columns.createdAt') },
  { key: 'actions', label: t('admin.redeem.columns.actions') }
])

const typeOptions = computed(() => [
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

const filterTypeOptions = computed(() => [
  { value: '', label: t('admin.redeem.allTypes') },
  ...typeOptions.value
])

const filterStatusOptions = computed(() => [
  { value: '', label: t('admin.redeem.allStatus') },
  { value: 'unused', label: t('admin.redeem.unused') },
  { value: 'used', label: t('admin.redeem.used') },
  { value: 'expired', label: t('admin.redeem.status.expired') }
])

let abortController: AbortController | null = null

const buildFilters = () => ({
  type: (filters.type || undefined) as RedeemCodeType | undefined,
  status: (filters.status || undefined) as 'used' | 'expired' | 'unused' | undefined,
  search: searchQuery.value || undefined
})

const loadBatches = async () => {
  if (abortController) abortController.abort()
  const currentController = new AbortController()
  abortController = currentController
  loading.value = true
  try {
    const response = await adminAPI.redeem.listBatches(
      pagination.page,
      pagination.page_size,
      buildFilters(),
      { signal: currentController.signal }
    )
    if (currentController.signal.aborted) return
    batches.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    appStore.showError(t('admin.redeem.failedToLoad'))
    console.error('Error loading redeem batches:', error)
  } finally {
    if (abortController === currentController && !currentController.signal.aborted) {
      loading.value = false
      abortController = null
    }
  }
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadBatches()
  }, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadBatches()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadBatches()
}

const openGenerateDialog = () => {
  showGenerateDialog.value = true
}

const handleGenerateCodes = async () => {
  if (generateForm.type === 'subscription' && !generateForm.group_id) {
    appStore.showError(t('admin.redeem.groupRequired'))
    return
  }

  generating.value = true
  try {
    const result = await adminAPI.redeem.generate(
      generateForm.count,
      generateForm.type,
      generateForm.value,
      generateForm.type === 'subscription' ? generateForm.group_id : undefined,
      generateForm.type === 'subscription' ? generateForm.validity_days : undefined,
      generateForm.batch_redeem_limit_per_user,
      generateForm.expires_at ? new Date(generateForm.expires_at).toISOString() : undefined
    )
    // Overlay right panel; keep dialog open for continuous generation
    generatedCodes.value = result
    copiedAll.value = false
    loadBatches()
    appStore.showSuccess(t('admin.redeem.codesCreated', { count: result.length }))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToGenerate'))
    console.error('Error generating codes:', error)
  } finally {
    generating.value = false
  }
}

const copyGeneratedCodes = async () => {
  try {
    await navigator.clipboard.writeText(generatedCodesText.value)
    copiedAll.value = true
    setTimeout(() => {
      copiedAll.value = false
    }, 2000)
  } catch {
    appStore.showError(t('admin.redeem.failedToCopy'))
  }
}

const downloadGeneratedCodes = () => {
  const blob = new Blob([generatedCodesText.value], { type: 'text/plain' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.txt`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}

const copyToClipboard = async (text: string) => {
  await clipboardCopy(text, t('admin.redeem.copied'))
}

const openBatchDetail = async (batch: RedeemCodeBatch) => {
  detailBatch.value = batch
  showDetail.value = true
  detailLoading.value = true
  detailCodes.value = []
  try {
    detailCodes.value = await adminAPI.redeem.listBatchCodes(batch.batch_key)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToLoad'))
  } finally {
    detailLoading.value = false
  }
}

const closeBatchDetail = () => {
  showDetail.value = false
  detailBatch.value = null
  detailCodes.value = []
}

const copyBatchUnused = async (batch: RedeemCodeBatch) => {
  try {
    const codes = await adminAPI.redeem.listBatchCodes(batch.batch_key)
    const unused = codes.filter((c) => c.status === 'unused').map((c) => c.code)
    if (!unused.length) {
      appStore.showInfo(t('admin.redeem.noUnusedCodes'))
      return
    }
    await navigator.clipboard.writeText(unused.join('\n'))
    appStore.showSuccess(t('admin.redeem.copiedUnused', { count: unused.length }))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToCopy'))
  }
}

const copyDetailUnused = async () => {
  const unused = detailUnusedCodes.value.map((c) => c.code)
  if (!unused.length) return
  try {
    await navigator.clipboard.writeText(unused.join('\n'))
    appStore.showSuccess(t('admin.redeem.copiedUnused', { count: unused.length }))
  } catch {
    appStore.showError(t('admin.redeem.failedToCopy'))
  }
}

const confirmDeleteBatchUnused = (batch: RedeemCodeBatch) => {
  pendingDeleteBatch.value = batch
  showDeleteBatchDialog.value = true
}

const executeDeleteBatchUnused = async () => {
  const batch = pendingDeleteBatch.value
  if (!batch) return
  try {
    const result = await adminAPI.redeem.deleteBatchUnused(batch.batch_key)
    appStore.showSuccess(t('admin.redeem.codesDeleted', { count: result.deleted }))
    showDeleteBatchDialog.value = false
    pendingDeleteBatch.value = null
    if (detailBatch.value?.batch_key === batch.batch_key) {
      await openBatchDetail(batch)
    }
    loadBatches()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToDelete'))
  }
}

const deleteDetailUnused = async () => {
  if (!detailBatch.value) return
  pendingDeleteBatch.value = detailBatch.value
  showDeleteBatchDialog.value = true
}

const deleteSingleCode = async (code: RedeemCode) => {
  try {
    await adminAPI.redeem.delete(code.id)
    appStore.showSuccess(t('admin.redeem.codeDeleted'))
    detailCodes.value = detailCodes.value.filter((c) => c.id !== code.id)
    loadBatches()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToDelete'))
  }
}

const handleExportCodes = async () => {
  try {
    const blob = await adminAPI.redeem.exportCodes(buildFilters())
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    appStore.showSuccess(t('admin.redeem.codesExported'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToExport'))
  }
}

const loadSubscriptionGroups = async () => {
  try {
    subscriptionGroups.value = await adminAPI.groups.getAll()
  } catch (error) {
    console.error('Error loading subscription groups:', error)
  }
}

onMounted(() => {
  loadBatches()
  loadSubscriptionGroups()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
