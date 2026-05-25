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
              <p class="input-hint">{{ t('admin.distribution.settings.subscriptionDiscountHint') }}</p>
            </div>
          </div>
          <div class="mt-4">
            <label class="input-label">{{ t('admin.distribution.settings.apiKeyGroups') }}</label>
            <div class="max-h-44 overflow-y-auto rounded-lg border border-gray-200 p-3 dark:border-dark-700">
              <label
                v-for="group in standardGroups"
                :key="group.id"
                class="flex cursor-pointer items-start gap-3 rounded-md px-2 py-2 hover:bg-gray-50 dark:hover:bg-dark-900"
              >
                <input
                  v-model="settingsForm.api_key_group_ids"
                  type="checkbox"
                  class="mt-1"
                  :value="group.id"
                />
                <span class="min-w-0">
                  <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
                  <span class="block text-xs text-gray-500 dark:text-dark-400">{{ group.platform }} · ID {{ group.id }}</span>
                </span>
              </label>
              <p v-if="standardGroups.length === 0" class="py-4 text-center text-sm text-gray-500 dark:text-dark-400">
                {{ t('admin.distribution.settings.noApiKeyGroups') }}
              </p>
            </div>
            <p class="input-hint">{{ t('admin.distribution.settings.apiKeyGroupsHint') }}</p>
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

      <div class="card p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.distribution.agentAccounts') }}</h3>
          <div class="flex flex-wrap items-center gap-2">
            <input v-model.trim="search" class="input w-64" :placeholder="t('admin.distribution.searchPlaceholder')" @input="handleSearch" />
            <button class="btn btn-secondary btn-sm" :disabled="applicationsLoading || walletsLoading" @click="loadAgentTables">{{ t('common.refresh') }}</button>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1180px] text-left text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="px-3 py-2 font-medium">{{ t('admin.distribution.columns.user') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('common.status') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.distribution.columns.contact') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.distribution.columns.reason') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.stats.balance') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.stats.recharged') }}</th>
                <th class="px-3 py-2 font-medium text-right">{{ t('distribution.stats.spent') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.distribution.columns.createdAt') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="agent in agentRows" :key="agent.user_id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                <td class="px-3 py-3">
                  <p class="font-medium text-gray-900 dark:text-white">{{ agent.user_email || '-' }}</p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">ID {{ agent.user_id }} / {{ agent.username || '-' }}</p>
                </td>
                <td class="px-3 py-3">
                  <span class="badge" :class="statusBadgeClass(agent.status)">{{ t(`distribution.status.${agent.status}`) }}</span>
                </td>
                <td class="px-3 py-3"><span class="block max-w-36 truncate">{{ agent.contact || '-' }}</span></td>
                <td class="px-3 py-3"><span class="block max-w-56 truncate">{{ agent.reason || '-' }}</span></td>
                <td class="px-3 py-3 text-right font-medium">{{ agent.wallet ? formatCurrency(agent.wallet.balance, 'CNY') : '-' }}</td>
                <td class="px-3 py-3 text-right">{{ agent.wallet ? formatCurrency(agent.wallet.total_recharged, 'CNY') : '-' }}</td>
                <td class="px-3 py-3 text-right">{{ agent.wallet ? formatCurrency(agent.wallet.total_spent, 'CNY') : '-' }}</td>
                <td class="px-3 py-3">{{ formatDateTime(agent.created_at) }}</td>
                <td class="px-3 py-3">
                  <div class="flex items-center gap-2">
                    <button class="btn btn-primary btn-sm" :disabled="agent.status === 'approved' || reviewing" @click="openReview(agent, true)">{{ t('admin.distribution.approve') }}</button>
                    <button class="btn btn-danger btn-sm" :disabled="agent.status === 'rejected' || reviewing" @click="openReview(agent, false)">{{ t('admin.distribution.reject') }}</button>
                    <button v-if="agent.wallet" class="btn btn-secondary btn-sm" @click="openAdjust(agent.wallet)">{{ t('admin.distribution.adjust') }}</button>
                    <button v-if="agent.wallet" class="btn btn-secondary btn-sm" @click="openRates(agent.wallet)">{{ t('admin.distribution.rates') }}</button>
                    <button v-if="agent.wallet" class="btn btn-secondary btn-sm" @click="toggleWallet(agent.wallet)">{{ agent.wallet.status === 'active' ? t('admin.distribution.freeze') : t('admin.distribution.unfreeze') }}</button>
                    <button v-if="agent.wallet" class="btn btn-secondary btn-sm" @click="filterLedger(agent.user_id)">{{ t('distribution.ledger.title') }}</button>
                    <button v-if="agent.wallet" class="btn btn-secondary btn-sm" @click="filterAssets(agent.user_id)">{{ t('distribution.assets.title') }}</button>
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
              <p class="input-hint">{{ t('admin.distribution.settings.subscriptionDiscountHint') }} {{ t('admin.distribution.ratesHint') }}</p>
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
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import distributionAdminAPI from '@/api/admin/distribution'
import groupsAdminAPI from '@/api/admin/groups'
import type { AdminGroup, DistributionAgentApplication, DistributionAsset, DistributionWallet, DistributionWalletLedgerEntry } from '@/types'
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
const standardGroups = ref<AdminGroup[]>([])
const ledgerItems = ref<DistributionWalletLedgerEntry[]>([])
const assetItems = ref<DistributionAsset[]>([])
const settingsForm = reactive({ rmb_per_usd: 0.5, subscription_discount: 0.75, api_key_group_ids: [] as number[] })
const applicationPagination = reactive({ page: 1, page_size: 20, total: 0 })
const walletPagination = reactive({ page: 1, page_size: 50, total: 0 })
const ledgerFilter = reactive({ user_id: 0 })
const assetFilter = reactive({ user_id: 0 })
const reviewDialog = reactive({ open: false, approved: true, note: '', application: null as DistributionAgentApplication | null })
const adjustDialog = reactive({ open: false, amount: 0, note: '', wallet: null as DistributionWallet | null })
const ratesDialog = reactive({ open: false, rmb_per_usd_override: 0, subscription_discount_override: 0, wallet: null as DistributionWallet | null })

const agentRows = computed(() => applications.value.map((application) => ({
  ...application,
  wallet: wallets.value.find((wallet) => wallet.user_id === application.user_id) ?? null,
})))

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
  settingsForm.api_key_group_ids = settings.api_key_group_ids ?? []
}

async function loadGroups(): Promise<void> {
  const groups = await groupsAdminAPI.getAll()
  standardGroups.value = groups.filter(group => group.status === 'active' && group.subscription_type === 'standard')
}

async function saveSettings(): Promise<void> {
  settingsSaving.value = true
  try {
    const settings = await distributionAdminAPI.updateSettings({
      rmb_per_usd: settingsForm.rmb_per_usd,
      subscription_discount: settingsForm.subscription_discount,
      api_key_group_ids: [...settingsForm.api_key_group_ids],
    })
    settingsForm.rmb_per_usd = settings.rmb_per_usd
    settingsForm.subscription_discount = settings.subscription_discount
    settingsForm.api_key_group_ids = settings.api_key_group_ids ?? []
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

async function loadAgentTables(): Promise<void> {
  await Promise.all([loadApplications(), loadWallets()])
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
    void loadAgentTables()
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
    await loadAgentTables()
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
    await Promise.all([loadAgentTables(), loadLedger()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.distribution.adjustFailed')))
  } finally {
    adjusting.value = false
  }
}

async function toggleWallet(wallet: DistributionWallet): Promise<void> {
  try {
    await distributionAdminAPI.updateWalletStatus(wallet.user_id, { frozen: wallet.status === 'active' })
    await loadAgentTables()
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
    await Promise.all([loadAssets(), loadAgentTables(), loadLedger()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.assets.voidFailed')))
  } finally {
    voidingAssetId.value = null
  }
}

onMounted(() => {
  void Promise.all([loadSettings(), loadGroups(), loadAgentTables(), loadLedger(), loadAssets()])
})
</script>
