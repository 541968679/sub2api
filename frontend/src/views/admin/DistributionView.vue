<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="grid gap-4 lg:grid-cols-2">
        <form class="card p-5" @submit.prevent="saveSettings">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.distribution.settings.title') }}</h3>
          <div class="mt-4 grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.distribution.settings.rmbPerUsd') }}</label>
              <input v-model.number="settingsForm.rmb_per_usd" type="number" min="0" step="0.01" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.distribution.settings.subscriptionDiscount') }}</label>
              <input v-model.number="settingsForm.subscription_discount" type="number" min="0" max="1" step="0.01" class="input" />
            </div>
          </div>
          <div class="mt-4 flex justify-end">
            <button class="btn btn-primary" :disabled="settingsSaving">{{ settingsSaving ? t('common.saving') : t('common.save') }}</button>
          </div>
        </form>

        <div class="card p-5">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.distribution.quickStats') }}</h3>
          <div class="mt-4 grid grid-cols-2 gap-4">
            <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-900">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.distribution.wallets') }}</p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ walletPagination.total }}</p>
            </div>
            <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-900">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.distribution.applications') }}</p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ applicationPagination.total }}</p>
            </div>
          </div>
        </div>
      </div>

      <TablePageLayout>
        <template #filters>
          <div class="flex flex-wrap items-center gap-3">
            <input v-model.trim="search" class="input flex-1 sm:max-w-72" :placeholder="t('admin.distribution.searchPlaceholder')" @input="handleSearch" />
            <button class="btn btn-secondary" :disabled="applicationsLoading" @click="loadApplications">{{ t('common.refresh') }}</button>
          </div>
        </template>

        <template #table>
          <DataTable :columns="applicationColumns" :data="applications" :loading="applicationsLoading">
            <template #cell-user="{ row }">
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ row.user_email || '-' }}</p>
                <p class="text-xs text-gray-500 dark:text-dark-400">ID {{ row.user_id }} / {{ row.username || '-' }}</p>
              </div>
            </template>
            <template #cell-status="{ value }">
              <span class="badge" :class="statusBadgeClass(String(value))">{{ t(`distribution.status.${value}`) }}</span>
            </template>
            <template #cell-reason="{ value }"><span class="block max-w-md truncate">{{ value || '-' }}</span></template>
            <template #cell-created_at="{ value }">{{ formatDateTime(value) }}</template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-2">
                <button class="btn btn-primary btn-sm" :disabled="row.status === 'approved' || reviewing" @click="openReview(row, true)">{{ t('admin.distribution.approve') }}</button>
                <button class="btn btn-danger btn-sm" :disabled="row.status === 'rejected' || reviewing" @click="openReview(row, false)">{{ t('admin.distribution.reject') }}</button>
              </div>
            </template>
          </DataTable>
        </template>
      </TablePageLayout>

      <div class="card p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.distribution.wallets') }}</h3>
          <button class="btn btn-secondary btn-sm" :disabled="walletsLoading" @click="loadWallets">{{ t('common.refresh') }}</button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[920px] text-left text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="px-3 py-2 font-medium">{{ t('admin.distribution.columns.user') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.stats.balance') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.stats.recharged') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.stats.spent') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('common.status') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="wallet in wallets" :key="wallet.id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                <td class="px-3 py-3">
                  <p class="font-medium text-gray-900 dark:text-white">{{ wallet.user_email || '-' }}</p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">ID {{ wallet.user_id }} / {{ wallet.username || '-' }}</p>
                </td>
                <td class="px-3 py-3 text-right font-medium">{{ formatCurrency(wallet.balance, 'CNY') }}</td>
                <td class="px-3 py-3 text-right">{{ formatCurrency(wallet.total_recharged, 'CNY') }}</td>
                <td class="px-3 py-3 text-right">{{ formatCurrency(wallet.total_spent, 'CNY') }}</td>
                <td class="px-3 py-3"><span class="badge" :class="wallet.status === 'active' ? 'badge-success' : 'badge-warning'">{{ wallet.status }}</span></td>
                <td class="px-3 py-3">
                  <div class="flex items-center gap-2">
                    <button class="btn btn-secondary btn-sm" @click="openAdjust(wallet)">{{ t('admin.distribution.adjust') }}</button>
                    <button class="btn btn-secondary btn-sm" @click="openRates(wallet)">{{ t('admin.distribution.rates') }}</button>
                    <button class="btn btn-secondary btn-sm" @click="toggleWallet(wallet)">{{ wallet.status === 'active' ? t('admin.distribution.freeze') : t('admin.distribution.unfreeze') }}</button>
                    <button class="btn btn-secondary btn-sm" @click="filterLedger(wallet.user_id)">{{ t('distribution.ledger.title') }}</button>
                    <button class="btn btn-secondary btn-sm" @click="filterAssets(wallet.user_id)">{{ t('distribution.assets.title') }}</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.assets.title') }}</h3>
          <div class="flex flex-wrap items-center gap-2">
            <input v-model.trim="assetSearch" class="input w-56" :placeholder="t('admin.distribution.assetSearchPlaceholder')" @input="handleAssetSearch" />
            <button v-if="assetFilter.user_id" class="btn btn-secondary btn-sm" @click="clearAssetFilter">{{ t('common.clear') }}</button>
            <button class="btn btn-secondary btn-sm" :disabled="assetsLoading" @click="loadAssets">{{ t('common.refresh') }}</button>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1120px] text-left text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="px-3 py-2 font-medium">{{ t('admin.distribution.columns.user') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('distribution.assets.columns.type') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('distribution.assets.columns.value') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.assets.columns.faceValue') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.assets.columns.cost') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('common.status') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('distribution.assets.columns.customer') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('distribution.assets.columns.createdAt') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="asset in assetItems" :key="asset.id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                <td class="px-3 py-3">
                  <p class="font-medium text-gray-900 dark:text-white">{{ asset.user_email || '-' }}</p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">ID {{ asset.user_id }} / {{ asset.username || '-' }}</p>
                </td>
                <td class="px-3 py-3">{{ assetTypeLabel(asset.asset_type) }}</td>
                <td class="px-3 py-3">
                  <p class="break-all font-mono text-xs text-gray-900 dark:text-white">{{ asset.display_value }}</p>
                  <p v-if="asset.package_url" class="mt-1 break-all text-xs text-gray-500 dark:text-dark-400">{{ asset.package_url }}</p>
                </td>
                <td class="px-3 py-3 text-right">{{ assetFaceValue(asset) }}</td>
                <td class="px-3 py-3 text-right">{{ formatCurrency(asset.cost_rmb, 'CNY') }}</td>
                <td class="px-3 py-3"><span class="badge" :class="assetStatusBadge(asset.status)">{{ assetStatusLabel(asset.status) }}</span></td>
                <td class="px-3 py-3">
                  <span v-if="asset.customer_user_id">ID {{ asset.customer_user_id }} / {{ asset.customer_email || '-' }}</span>
                  <span v-else>-</span>
                </td>
                <td class="px-3 py-3">{{ formatDateTime(asset.created_at) }}</td>
                <td class="px-3 py-3">
                  <button v-if="canVoidAsset(asset)" class="btn btn-danger btn-sm" :disabled="voidingAssetId === asset.id" @click="voidAsset(asset)">
                    {{ t('distribution.assets.void') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.ledger.title') }}</h3>
          <button class="btn btn-secondary btn-sm" :disabled="ledgerLoading" @click="loadLedger">{{ t('common.refresh') }}</button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[860px] text-left text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="px-3 py-2 font-medium">User ID</th>
                <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.action') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.ledger.columns.amount') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.ledger.columns.balanceAfter') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.note') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.createdAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in ledgerItems" :key="item.id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                <td class="px-3 py-3">{{ item.user_id }}</td>
                <td class="px-3 py-3">{{ actionLabel(item.action) }}</td>
                <td class="px-3 py-3 text-right font-medium">{{ formatCurrency(item.amount, 'CNY') }}</td>
                <td class="px-3 py-3 text-right">{{ formatCurrency(item.balance_after, 'CNY') }}</td>
                <td class="px-3 py-3">{{ item.note || '-' }}</td>
                <td class="px-3 py-3">{{ formatDateTime(item.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="reviewDialog.open" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="closeReview"></div>
        <div class="relative z-10 w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ reviewDialog.approved ? t('admin.distribution.approveTitle') : t('admin.distribution.rejectTitle') }}</h2>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ reviewDialog.application?.user_email || `ID ${reviewDialog.application?.user_id ?? ''}` }}</p>
          <div class="mt-4">
            <label class="input-label">{{ t('admin.distribution.reviewNote') }}</label>
            <textarea v-model.trim="reviewDialog.note" class="input min-h-24"></textarea>
          </div>
          <div class="mt-6 flex justify-end gap-3">
            <button class="btn btn-secondary" @click="closeReview">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="reviewing" @click="submitReview">{{ reviewing ? t('common.processing') : t('common.confirm') }}</button>
          </div>
        </div>
      </div>

      <div v-if="adjustDialog.open" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="closeAdjust"></div>
        <div class="relative z-10 w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.distribution.adjust') }}</h2>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">ID {{ adjustDialog.wallet?.user_id }}</p>
          <div class="mt-4 space-y-4">
            <div>
              <label class="input-label">{{ t('admin.distribution.adjustAmount') }}</label>
              <input v-model.number="adjustDialog.amount" type="number" step="0.01" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('distribution.ledger.columns.note') }}</label>
              <textarea v-model.trim="adjustDialog.note" class="input min-h-24"></textarea>
            </div>
          </div>
          <div class="mt-6 flex justify-end gap-3">
            <button class="btn btn-secondary" @click="closeAdjust">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="adjusting" @click="submitAdjust">{{ adjusting ? t('common.processing') : t('common.confirm') }}</button>
          </div>
        </div>
      </div>

      <div v-if="ratesDialog.open" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="closeRates"></div>
        <div class="relative z-10 w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.distribution.rates') }}</h2>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">ID {{ ratesDialog.wallet?.user_id }}</p>
          <div class="mt-4 space-y-4">
            <div>
              <label class="input-label">{{ t('admin.distribution.settings.rmbPerUsd') }}</label>
              <input v-model.number="ratesDialog.rmb_per_usd_override" type="number" min="0" step="0.01" class="input" />
              <p class="input-hint">{{ t('admin.distribution.ratesHint') }}</p>
            </div>
            <div>
              <label class="input-label">{{ t('admin.distribution.settings.subscriptionDiscount') }}</label>
              <input v-model.number="ratesDialog.subscription_discount_override" type="number" min="0" max="1" step="0.01" class="input" />
              <p class="input-hint">{{ t('admin.distribution.ratesHint') }}</p>
            </div>
          </div>
          <div class="mt-6 flex justify-end gap-3">
            <button class="btn btn-secondary" @click="closeRates">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="ratesSaving" @click="submitRates">{{ ratesSaving ? t('common.processing') : t('common.confirm') }}</button>
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
import distributionAdminAPI from '@/api/admin/distribution'
import type { DistributionAgentApplication, DistributionAsset, DistributionWallet, DistributionWalletLedgerEntry } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const settingsSaving = ref(false)
const applicationsLoading = ref(false)
const walletsLoading = ref(false)
const ledgerLoading = ref(false)
const assetsLoading = ref(false)
const reviewing = ref(false)
const adjusting = ref(false)
const ratesSaving = ref(false)
const search = ref('')
const assetSearch = ref('')
const voidingAssetId = ref<number | null>(null)
const searchTimer = ref<number | null>(null)
const assetSearchTimer = ref<number | null>(null)
const applications = ref<DistributionAgentApplication[]>([])
const wallets = ref<DistributionWallet[]>([])
const ledgerItems = ref<DistributionWalletLedgerEntry[]>([])
const assetItems = ref<DistributionAsset[]>([])
const settingsForm = reactive({ rmb_per_usd: 0.5, subscription_discount: 0.75 })
const applicationPagination = reactive({ page: 1, page_size: 20, total: 0 })
const walletPagination = reactive({ page: 1, page_size: 50, total: 0 })
const ledgerFilter = reactive({ user_id: 0 })
const assetFilter = reactive({ user_id: 0 })
const reviewDialog = reactive({ open: false, approved: true, note: '', application: null as DistributionAgentApplication | null })
const adjustDialog = reactive({ open: false, amount: 0, note: '', wallet: null as DistributionWallet | null })
const ratesDialog = reactive({ open: false, rmb_per_usd_override: 0, subscription_discount_override: 0, wallet: null as DistributionWallet | null })

const applicationColumns: Column[] = [
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

async function loadSettings(): Promise<void> {
  const settings = await distributionAdminAPI.getSettings()
  settingsForm.rmb_per_usd = settings.rmb_per_usd
  settingsForm.subscription_discount = settings.subscription_discount
}

async function saveSettings(): Promise<void> {
  settingsSaving.value = true
  try {
    const settings = await distributionAdminAPI.updateSettings({ ...settingsForm })
    settingsForm.rmb_per_usd = settings.rmb_per_usd
    settingsForm.subscription_discount = settings.subscription_discount
    appStore.showSuccess(t('common.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.saveSettingsFailed')))
  } finally {
    settingsSaving.value = false
  }
}

async function loadApplications(): Promise<void> {
  applicationsLoading.value = true
  try {
    const resp = await distributionAdminAPI.listApplications({ page: applicationPagination.page, page_size: applicationPagination.page_size, search: search.value })
    applications.value = resp.items ?? []
    applicationPagination.total = resp.total ?? 0
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.loadFailed')))
  } finally {
    applicationsLoading.value = false
  }
}

async function loadWallets(): Promise<void> {
  walletsLoading.value = true
  try {
    const resp = await distributionAdminAPI.listWallets({ page: walletPagination.page, page_size: walletPagination.page_size, search: search.value })
    wallets.value = resp.items ?? []
    walletPagination.total = resp.total ?? 0
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.loadWalletsFailed')))
  } finally {
    walletsLoading.value = false
  }
}

async function loadLedger(): Promise<void> {
  ledgerLoading.value = true
  try {
    const resp = await distributionAdminAPI.listLedger({ page: 1, page_size: 50, user_id: ledgerFilter.user_id || undefined })
    ledgerItems.value = resp.items ?? []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.loadLedgerFailed')))
  } finally {
    ledgerLoading.value = false
  }
}

async function loadAssets(): Promise<void> {
  assetsLoading.value = true
  try {
    const resp = await distributionAdminAPI.listAssets({ page: 1, page_size: 50, user_id: assetFilter.user_id || undefined, search: assetSearch.value })
    assetItems.value = resp.items ?? []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.loadAssetsFailed')))
  } finally {
    assetsLoading.value = false
  }
}

function handleSearch(): void {
  if (searchTimer.value) window.clearTimeout(searchTimer.value)
  searchTimer.value = window.setTimeout(() => {
    void Promise.all([loadApplications(), loadWallets()])
  }, 300)
}

function handleAssetSearch(): void {
  if (assetSearchTimer.value) window.clearTimeout(assetSearchTimer.value)
  assetSearchTimer.value = window.setTimeout(() => {
    void loadAssets()
  }, 300)
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
    await distributionAdminAPI.reviewApplication(reviewDialog.application.user_id, { approved: reviewDialog.approved, note: reviewDialog.note })
    appStore.showSuccess(t('admin.distribution.reviewSuccess'))
    closeReview()
    await Promise.all([loadApplications(), loadWallets()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.reviewFailed')))
  } finally {
    reviewing.value = false
  }
}

function openAdjust(wallet: DistributionWallet): void {
  adjustDialog.open = true
  adjustDialog.wallet = wallet
  adjustDialog.amount = 0
  adjustDialog.note = ''
}

function openRates(wallet: DistributionWallet): void {
  ratesDialog.open = true
  ratesDialog.wallet = wallet
  const app = applications.value.find(item => item.user_id === wallet.user_id)
  ratesDialog.rmb_per_usd_override = app?.rmb_per_usd_override ?? 0
  ratesDialog.subscription_discount_override = app?.subscription_discount_override ?? 0
}

function closeRates(): void {
  ratesDialog.open = false
  ratesDialog.wallet = null
}

async function submitRates(): Promise<void> {
  if (!ratesDialog.wallet || ratesSaving.value) return
  ratesSaving.value = true
  try {
    await distributionAdminAPI.updateAgentRates(ratesDialog.wallet.user_id, {
      rmb_per_usd_override: ratesDialog.rmb_per_usd_override > 0 ? ratesDialog.rmb_per_usd_override : null,
      subscription_discount_override: ratesDialog.subscription_discount_override > 0 ? ratesDialog.subscription_discount_override : null,
    })
    appStore.showSuccess(t('common.saved'))
    closeRates()
    await loadApplications()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.ratesFailed')))
  } finally {
    ratesSaving.value = false
  }
}

function closeAdjust(): void {
  adjustDialog.open = false
  adjustDialog.wallet = null
}

async function submitAdjust(): Promise<void> {
  if (!adjustDialog.wallet || adjusting.value) return
  adjusting.value = true
  try {
    await distributionAdminAPI.adjustWallet(adjustDialog.wallet.user_id, { amount: adjustDialog.amount, note: adjustDialog.note })
    appStore.showSuccess(t('common.saved'))
    closeAdjust()
    await Promise.all([loadWallets(), loadLedger()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.adjustFailed')))
  } finally {
    adjusting.value = false
  }
}

async function toggleWallet(wallet: DistributionWallet): Promise<void> {
  try {
    await distributionAdminAPI.updateWalletStatus(wallet.user_id, { frozen: wallet.status === 'active' })
    await loadWallets()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.statusFailed')))
  }
}

function filterLedger(userId: number): void {
  ledgerFilter.user_id = userId
  void loadLedger()
}

function filterAssets(userId: number): void {
  assetFilter.user_id = userId
  void loadAssets()
}

function clearAssetFilter(): void {
  assetFilter.user_id = 0
  void loadAssets()
}

function actionLabel(action: string): string {
  return t(`distribution.ledger.actions.${action}`, action)
}

function assetTypeLabel(type: string): string {
  return t(`distribution.assets.types.${type}`, type)
}

function assetStatusLabel(status: string): string {
  return t(`distribution.assets.status.${status}`, status)
}

function assetStatusBadge(status: string): string {
  if (status === 'active') return 'badge-success'
  if (status === 'used') return 'badge-primary'
  if (status === 'expired') return 'badge-warning'
  return 'badge-gray'
}

function assetFaceValue(asset: DistributionAsset): string {
  if (asset.asset_type === 'subscription_redeem_code') return formatCurrency(asset.face_value, 'CNY')
  return `$${(asset.quota_usd || asset.face_value || 0).toFixed(2)}`
}

function canVoidAsset(asset: DistributionAsset): boolean {
  return asset.status === 'active' && !asset.refunded_at
}

async function voidAsset(asset: DistributionAsset): Promise<void> {
  if (!canVoidAsset(asset) || voidingAssetId.value) return
  voidingAssetId.value = asset.id
  try {
    const result = await distributionAdminAPI.voidAsset(asset.id)
    appStore.showSuccess(t('distribution.assets.voidSuccess', { amount: formatCurrency(result.refund_rmb, 'CNY') }))
    await Promise.all([loadAssets(), loadWallets(), loadLedger()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.assets.voidFailed')))
  } finally {
    voidingAssetId.value = null
  }
}

onMounted(() => {
  void Promise.all([loadSettings(), loadApplications(), loadWallets(), loadLedger(), loadAssets()])
})
</script>
