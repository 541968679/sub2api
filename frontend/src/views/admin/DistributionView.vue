<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-72">
            <input
              v-model.trim="search"
              class="input"
              :placeholder="t('admin.distribution.searchPlaceholder')"
              @input="handleSearch"
            />
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="loadApplications">
            {{ t('common.refresh') }}
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="applications" :loading="loading">
          <template #cell-user="{ row }">
            <div>
              <p class="font-medium text-gray-900 dark:text-white">{{ row.user_email || '-' }}</p>
              <p class="text-xs text-gray-500 dark:text-dark-400">ID {{ row.user_id }} · {{ row.username || '-' }}</p>
            </div>
          </template>

          <template #cell-status="{ value }">
            <span class="badge" :class="statusBadgeClass(String(value))">
              {{ t(`distribution.status.${value}`) }}
            </span>
          </template>

          <template #cell-reason="{ value }">
            <span class="block max-w-md truncate">{{ value || '-' }}</span>
          </template>

          <template #cell-created_at="{ value }">
            {{ formatDateTime(value) }}
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button
                class="btn btn-primary btn-sm"
                :disabled="row.status === 'approved' || reviewing"
                @click="openReview(row, true)"
              >
                {{ t('admin.distribution.approve') }}
              </button>
              <button
                class="btn btn-danger btn-sm"
                :disabled="row.status === 'rejected' || reviewing"
                @click="openReview(row, false)"
              >
                {{ t('admin.distribution.reject') }}
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
          @update:page="changePage"
          @update:pageSize="changePageSize"
        />
      </template>
    </TablePageLayout>

    <Teleport to="body">
      <div v-if="reviewDialog.open" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="closeReview"></div>
        <div class="relative z-10 w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ reviewDialog.approved ? t('admin.distribution.approveTitle') : t('admin.distribution.rejectTitle') }}
          </h2>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
            {{ reviewDialog.application?.user_email || `ID ${reviewDialog.application?.user_id ?? ''}` }}
          </p>
          <div class="mt-4">
            <label class="input-label">{{ t('admin.distribution.reviewNote') }}</label>
            <textarea v-model.trim="reviewDialog.note" class="input min-h-24"></textarea>
          </div>
          <div class="mt-6 flex justify-end gap-3">
            <button class="btn btn-secondary" @click="closeReview">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="reviewing" @click="submitReview">
              {{ reviewing ? t('common.processing') : t('common.confirm') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import distributionAdminAPI from '@/api/admin/distribution'
import type { DistributionAgentApplication } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const reviewing = ref(false)
const search = ref('')
const searchTimer = ref<number | null>(null)
const applications = ref<DistributionAgentApplication[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const reviewDialog = reactive({
  open: false,
  approved: true,
  note: '',
  application: null as DistributionAgentApplication | null,
})

const columns: Column[] = [
  { key: 'user', label: t('admin.distribution.columns.user') },
  { key: 'status', label: t('common.status') },
  { key: 'contact', label: t('admin.distribution.columns.contact') },
  { key: 'reason', label: t('admin.distribution.columns.reason') },
  { key: 'created_at', label: t('admin.distribution.columns.createdAt') },
  { key: 'actions', label: t('common.actions') },
]

function statusBadgeClass(status: string): string {
  if (status === 'approved') return 'badge-success'
  if (status === 'rejected') return 'badge-danger'
  if (status === 'frozen') return 'badge-warning'
  return 'badge-gray'
}

async function loadApplications(): Promise<void> {
  loading.value = true
  try {
    const resp = await distributionAdminAPI.listApplications({
      page: pagination.page,
      page_size: pagination.page_size,
      search: search.value,
    })
    applications.value = resp.items ?? []
    pagination.total = resp.total ?? 0
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.loadFailed')))
  } finally {
    loading.value = false
  }
}

function handleSearch(): void {
  if (searchTimer.value) window.clearTimeout(searchTimer.value)
  searchTimer.value = window.setTimeout(() => {
    pagination.page = 1
    void loadApplications()
  }, 300)
}

function changePage(page: number): void {
  pagination.page = page
  void loadApplications()
}

function changePageSize(pageSize: number): void {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadApplications()
}

function openReview(application: DistributionAgentApplication, approved: boolean): void {
  reviewDialog.open = true
  reviewDialog.application = application
  reviewDialog.approved = approved
  reviewDialog.note = ''
}

function closeReview(): void {
  reviewDialog.open = false
  reviewDialog.application = null
  reviewDialog.note = ''
}

async function submitReview(): Promise<void> {
  if (!reviewDialog.application || reviewing.value) return
  reviewing.value = true
  try {
    await distributionAdminAPI.reviewApplication(reviewDialog.application.user_id, {
      approved: reviewDialog.approved,
      note: reviewDialog.note,
    })
    appStore.showSuccess(t('admin.distribution.reviewSuccess'))
    closeReview()
    await loadApplications()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.reviewFailed')))
  } finally {
    reviewing.value = false
  }
}

onMounted(() => {
  void loadApplications()
})
</script>
