<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else>
        <div class="grid gap-4 md:grid-cols-4">
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.status') }}</p>
            <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">{{ statusLabel }}</p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.balance') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatCurrency(summary?.wallet?.balance ?? 0, 'CNY') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.recharged') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(summary?.wallet?.total_recharged ?? 0, 'CNY') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.spent') }}</p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formatCurrency(summary?.wallet?.total_spent ?? 0, 'CNY') }}
            </p>
          </div>
        </div>

        <div v-if="!summary?.application" class="card p-6">
          <div class="grid gap-5 lg:grid-cols-[1.15fr_0.85fr]">
            <div>
              <p class="text-xs font-semibold uppercase text-primary-600 dark:text-primary-400">
                {{ t('distribution.intro.eyebrow') }}
              </p>
              <h3 class="mt-2 text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.intro.title') }}</h3>
              <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('distribution.intro.description') }}</p>
            </div>
            <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('distribution.intro.benefitsTitle') }}</p>
              <div class="mt-3 space-y-3">
                <div v-for="item in benefitItems" :key="item.title" class="flex gap-3">
                  <span class="mt-1 h-2 w-2 shrink-0 rounded-full bg-primary-500"></span>
                  <div>
                    <p class="text-sm font-medium text-gray-900 dark:text-white">{{ item.title }}</p>
                    <p class="mt-0.5 text-sm leading-5 text-gray-500 dark:text-dark-400">{{ item.description }}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="mt-6 border-t border-gray-100 pt-6 dark:border-dark-800">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.apply.title') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.apply.description') }}</p>
          </div>
          <form class="mt-5 space-y-4" @submit.prevent="submitApplication">
            <div>
              <label class="input-label">{{ t('distribution.apply.contact') }}</label>
              <input v-model.trim="applyForm.contact" class="input" maxlength="200" :placeholder="t('distribution.apply.contactPlaceholder')" />
            </div>
            <div>
              <label class="input-label">{{ t('distribution.apply.reason') }}</label>
              <textarea v-model.trim="applyForm.reason" class="input min-h-28" maxlength="1000" :placeholder="t('distribution.apply.reasonPlaceholder')"></textarea>
            </div>
            <button class="btn btn-primary" :disabled="submitting">
              {{ submitting ? t('common.submitting') : t('distribution.apply.submit') }}
            </button>
          </form>
        </div>

        <div v-else-if="!isApproved" class="card p-6">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.application.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('distribution.application.submittedAt') }} {{ formatDateTime(summary.application.created_at) }}
              </p>
              <p v-if="summary.application.admin_note" class="mt-3 text-sm text-gray-700 dark:text-gray-300">
                {{ t('distribution.application.adminNote') }}: {{ summary.application.admin_note }}
              </p>
            </div>
            <span class="badge" :class="statusBadgeClass">{{ statusLabel }}</span>
          </div>
        </div>

        <div v-if="isApproved" class="card p-6">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.tutorial.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.tutorial.description') }}</p>
            </div>
            <span class="badge badge-success">{{ statusLabel }}</span>
          </div>
          <div class="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            <div
              v-for="(step, index) in tutorialSteps"
              :key="step.title"
              class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"
            >
              <div class="flex items-start gap-3">
                <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary-100 text-sm font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-200">
                  {{ index + 1 }}
                </span>
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ step.title }}</p>
                  <p class="mt-1 text-sm leading-5 text-gray-500 dark:text-dark-400">{{ step.description }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="isApproved" class="grid gap-4 lg:grid-cols-3">
          <form class="card p-5" @submit.prevent="generateBalanceCode">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.generate.balanceTitle') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ balanceCostHint }}</p>
            <div class="mt-4 space-y-3">
              <div>
                <label class="input-label">{{ t('distribution.generate.valueUsd') }}</label>
                <input v-model.number="balanceForm.value_usd" type="number" min="0" step="0.01" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.note') }}</label>
                <input v-model.trim="balanceForm.note" class="input" maxlength="200" />
              </div>
              <button class="btn btn-primary w-full" :disabled="generating.balance">
                {{ generating.balance ? t('common.processing') : t('distribution.generate.createCode') }}
              </button>
            </div>
          </form>

          <form class="card p-5" @submit.prevent="generateSubscriptionCode">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.generate.subscriptionTitle') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ subscriptionCostHint }}</p>
            <div class="mt-4 space-y-3">
              <div>
                <label class="input-label">{{ t('distribution.generate.subscriptionPlan') }}</label>
                <select v-model.number="subscriptionForm.plan_id" class="input">
                  <option :value="0">{{ t('distribution.generate.selectPlan') }}</option>
                  <option v-for="plan in subscriptionPlans" :key="plan.id" :value="plan.id">
                    {{ plan.name }} - {{ formatCurrency(plan.price, 'CNY') }} / {{ plan.validity_days }}{{ plan.validity_unit }}
                  </option>
                </select>
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.note') }}</label>
                <input v-model.trim="subscriptionForm.note" class="input" maxlength="200" />
              </div>
              <button class="btn btn-primary w-full" :disabled="generating.subscription">
                {{ generating.subscription ? t('common.processing') : t('distribution.generate.createCode') }}
              </button>
            </div>
          </form>

          <form class="card p-5" @submit.prevent="generateApiKey">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.generate.apiTitle') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ apiCostHint }}</p>
            <div class="mt-4 space-y-3">
              <div>
                <label class="input-label">{{ t('common.name') }}</label>
                <input v-model.trim="apiForm.name" class="input" maxlength="100" />
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.quotaUsd') }}</label>
                <input v-model.number="apiForm.quota_usd" type="number" min="0" step="0.01" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.group') }}</label>
                <select v-model.number="apiForm.group_id" class="input">
                  <option :value="0">{{ t('distribution.generate.selectGroup') }}</option>
                  <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
                </select>
                <p v-if="groups.length === 0" class="input-hint">{{ t('distribution.generate.noApiKeyGroups') }}</p>
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.expiresInDays') }}</label>
                <input v-model.number="apiForm.expires_in_days" type="number" min="0" step="1" class="input" />
              </div>
              <button class="btn btn-primary w-full" :disabled="generating.api">
                {{ generating.api ? t('common.processing') : t('distribution.generate.createApiKey') }}
              </button>
            </div>
          </form>
        </div>

        <div v-if="isApproved" class="card p-6">
          <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div class="inline-flex w-full rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-700 dark:bg-dark-900 sm:w-auto">
              <button
                type="button"
                class="flex-1 rounded-md px-3 py-2 text-sm font-medium transition sm:flex-none"
                :class="activeHistoryTab === 'assets' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-800 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'"
                @click="activeHistoryTab = 'assets'"
              >
                {{ t('distribution.assets.title') }}
              </button>
              <button
                type="button"
                class="flex-1 rounded-md px-3 py-2 text-sm font-medium transition sm:flex-none"
                :class="activeHistoryTab === 'ledger' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-800 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'"
                @click="activeHistoryTab = 'ledger'"
              >
                {{ t('distribution.ledger.title') }}
              </button>
            </div>

            <div v-if="activeHistoryTab === 'assets'" class="flex w-full flex-col gap-2 sm:flex-row sm:items-center xl:w-auto">
              <input
                v-model.trim="assetSearch"
                class="input h-9 w-full sm:w-72"
                :placeholder="t('distribution.assets.searchPlaceholder')"
                @input="handleAssetSearch"
              />
              <button v-if="assetSearch" class="btn btn-secondary btn-sm" @click="clearAssetSearch">{{ t('common.clear') }}</button>
              <button class="btn btn-secondary btn-sm" :disabled="assetsLoading" @click="loadAssets">{{ t('common.refresh') }}</button>
            </div>
            <button v-else class="btn btn-secondary btn-sm self-start xl:self-auto" :disabled="ledgerLoading" @click="loadLedger">{{ t('common.refresh') }}</button>
          </div>

          <div v-if="activeHistoryTab === 'assets' && generatedItems.length > 0" class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-800">
            <div class="mb-3 flex items-center justify-between gap-3">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('distribution.generated.latestTitle') }}</p>
              <button class="btn btn-secondary btn-sm" @click="generatedItems = []">{{ t('common.clear') }}</button>
            </div>
            <div class="grid gap-3 xl:grid-cols-2">
              <div v-for="item in generatedItems" :key="item.value" class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/70">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div class="min-w-0">
                    <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ item.label }}</p>
                    <pre v-if="item.multiline" class="mt-1 max-h-36 overflow-auto whitespace-pre-wrap break-all rounded-md bg-white p-3 font-mono text-xs font-medium text-gray-900 dark:bg-dark-800 dark:text-white">{{ item.value }}</pre>
                    <p v-else class="mt-1 break-all font-mono text-sm font-medium text-gray-900 dark:text-white">{{ item.value }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.meta }}</p>
                  </div>
                  <button class="btn btn-secondary btn-sm shrink-0" @click="copy(item.copyText || item.value)">{{ t('common.copy') }}</button>
                </div>
              </div>
            </div>
          </div>

          <div v-if="activeHistoryTab === 'assets'" class="mt-5">
            <div v-if="assets.items.length === 0" class="rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ assetSearch ? t('distribution.assets.searchEmpty') : t('distribution.assets.empty') }}
            </div>
            <div v-else class="overflow-x-auto">
              <table class="w-full min-w-[980px] text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
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
                  <tr v-for="asset in assets.items" :key="asset.id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                    <td class="px-3 py-3">{{ assetTypeLabel(asset.asset_type) }}</td>
                    <td class="px-3 py-3">
                      <p class="break-all font-mono text-xs text-gray-900 dark:text-white">{{ asset.display_value }}</p>
                      <p v-if="asset.package_url" class="mt-1 break-all text-xs text-gray-500 dark:text-dark-400">{{ asset.package_url }}</p>
                    </td>
                    <td class="px-3 py-3 text-right">{{ assetFaceValue(asset) }}</td>
                    <td class="px-3 py-3 text-right">{{ formatCurrency(asset.cost_rmb, 'CNY') }}</td>
                    <td class="px-3 py-3"><span class="badge" :class="assetStatusBadge(asset.status)">{{ assetStatusLabel(asset.status) }}</span></td>
                    <td class="px-3 py-3">{{ asset.customer_email || '-' }}</td>
                    <td class="px-3 py-3">{{ formatDateTime(asset.created_at) }}</td>
                    <td class="px-3 py-3">
                      <div class="flex items-center gap-2">
                        <button class="btn btn-secondary btn-sm" @click="copy(assetCopyText(asset))">{{ t('common.copy') }}</button>
                        <button v-if="canVoidAsset(asset)" class="btn btn-danger btn-sm" :disabled="voidingAssetId === asset.id" @click="voidAsset(asset)">
                          {{ t('distribution.assets.void') }}
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div v-else class="mt-5">
            <div v-if="ledger.items.length === 0" class="rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('distribution.ledger.empty') }}
            </div>
            <div v-else class="overflow-x-auto">
              <table class="w-full min-w-[760px] text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                    <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.action') }}</th>
                    <th class="px-3 py-2 font-medium text-right">{{ t('distribution.ledger.columns.amount') }}</th>
                    <th class="px-3 py-2 font-medium text-right">{{ t('distribution.ledger.columns.balanceAfter') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.note') }}</th>
                    <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.createdAt') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in ledger.items" :key="item.id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                    <td class="px-3 py-3 text-gray-900 dark:text-white">{{ actionLabel(item.action) }}</td>
                    <td class="px-3 py-3 text-right font-medium">{{ formatCurrency(item.amount, 'CNY') }}</td>
                    <td class="px-3 py-3 text-right">{{ formatCurrency(item.balance_after, 'CNY') }}</td>
                    <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ item.note || '-' }}</td>
                    <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ formatDateTime(item.created_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import distributionAPI from '@/api/distribution'
import { paymentAPI } from '@/api/payment'
import type { DistributionAsset, DistributionSummary, DistributionWalletLedgerEntry, Group } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const ledgerLoading = ref(false)
const assetsLoading = ref(false)
const voidingAssetId = ref<number | null>(null)
const summary = ref<DistributionSummary | null>(null)
const groups = ref<Group[]>([])
const subscriptionPlans = ref<SubscriptionPlan[]>([])
const generatedItems = ref<Array<{ label: string; value: string; meta: string; copyText?: string; multiline?: boolean }>>([])
const activeHistoryTab = ref<'assets' | 'ledger'>('assets')
const assetSearch = ref('')
const assetSearchTimer = ref<number | null>(null)
const ledger = reactive({ items: [] as DistributionWalletLedgerEntry[] })
const assets = reactive({ items: [] as DistributionAsset[] })
const applyForm = reactive({ contact: '', reason: '' })
const balanceForm = reactive({ value_usd: 10, note: '' })
const subscriptionForm = reactive({ plan_id: 0, note: '' })
const apiForm = reactive({ name: 'Distribution API Key', quota_usd: 10, group_id: 0, expires_in_days: 0 })
const generating = reactive({ balance: false, subscription: false, api: false })
const benefitItems = computed(() => [
  {
    title: t('distribution.intro.benefits.lowCost.title'),
    description: t('distribution.intro.benefits.lowCost.description'),
  },
  {
    title: t('distribution.intro.benefits.fastDelivery.title'),
    description: t('distribution.intro.benefits.fastDelivery.description'),
  },
  {
    title: t('distribution.intro.benefits.management.title'),
    description: t('distribution.intro.benefits.management.description'),
  },
])
const tutorialSteps = computed(() => [
  {
    title: t('distribution.tutorial.steps.recharge.title'),
    description: t('distribution.tutorial.steps.recharge.description'),
  },
  {
    title: t('distribution.tutorial.steps.choose.title'),
    description: t('distribution.tutorial.steps.choose.description'),
  },
  {
    title: t('distribution.tutorial.steps.generate.title'),
    description: t('distribution.tutorial.steps.generate.description'),
  },
  {
    title: t('distribution.tutorial.steps.deliver.title'),
    description: t('distribution.tutorial.steps.deliver.description'),
  },
  {
    title: t('distribution.tutorial.steps.track.title'),
    description: t('distribution.tutorial.steps.track.description'),
  },
  {
    title: t('distribution.tutorial.steps.void.title'),
    description: t('distribution.tutorial.steps.void.description'),
  },
])

const isApproved = computed(() => summary.value?.application?.status === 'approved' && summary.value?.wallet?.status === 'active')
const settings = computed(() => summary.value?.settings ?? { rmb_per_usd: 0.5, subscription_discount: 0.75 })
const selectedSubscriptionPlan = computed(() => subscriptionPlans.value.find((plan) => plan.id === subscriptionForm.plan_id) ?? null)

const statusLabel = computed(() => {
  const status = summary.value?.application?.status
  if (summary.value?.wallet?.status === 'frozen') return t('distribution.status.frozen')
  return status ? t(`distribution.status.${status}`) : t('distribution.status.notApplied')
})

const statusBadgeClass = computed(() => {
  if (summary.value?.wallet?.status === 'frozen') return 'badge-warning'
  switch (summary.value?.application?.status) {
    case 'approved':
      return 'badge-success'
    case 'rejected':
      return 'badge-danger'
    case 'frozen':
      return 'badge-warning'
    default:
      return 'badge-gray'
  }
})

const balanceCostHint = computed(() => t('distribution.generate.balanceHint', {
  cost: formatCurrency((balanceForm.value_usd || 0) * settings.value.rmb_per_usd, 'CNY'),
}))
const subscriptionCostHint = computed(() => t('distribution.generate.subscriptionHint', {
  price: formatCurrency(selectedSubscriptionPlan.value?.price ?? 0, 'CNY'),
  discount: formatDiscount(settings.value.subscription_discount),
  cost: formatCurrency((selectedSubscriptionPlan.value?.price ?? 0) * settings.value.subscription_discount, 'CNY'),
}))
const apiCostHint = computed(() => t('distribution.generate.apiHint', {
  cost: formatCurrency((apiForm.quota_usd || 0) * settings.value.rmb_per_usd, 'CNY'),
}))

async function loadSummary(): Promise<void> {
  summary.value = await distributionAPI.getSummary()
}

async function loadGroups(): Promise<void> {
  groups.value = await distributionAPI.listApiKeyGroups()
}

async function loadSubscriptionPlans(): Promise<void> {
  const { data } = await paymentAPI.getPlans()
  subscriptionPlans.value = data ?? []
}

async function loadLedger(): Promise<void> {
  ledgerLoading.value = true
  try {
    const resp = await distributionAPI.listLedger(1, 20)
    ledger.items = resp.items ?? []
  } catch {
    ledger.items = []
  } finally {
    ledgerLoading.value = false
  }
}

async function loadAssets(): Promise<void> {
  assetsLoading.value = true
  try {
    const resp = await distributionAPI.listAssets({ page: 1, page_size: 50, search: assetSearch.value || undefined })
    assets.items = resp.items ?? []
  } catch {
    assets.items = []
  } finally {
    assetsLoading.value = false
  }
}

function handleAssetSearch(): void {
  if (assetSearchTimer.value) window.clearTimeout(assetSearchTimer.value)
  assetSearchTimer.value = window.setTimeout(() => {
    void loadAssets()
  }, 300)
}

function clearAssetSearch(): void {
  if (assetSearchTimer.value) window.clearTimeout(assetSearchTimer.value)
  assetSearch.value = ''
  void loadAssets()
}

async function submitApplication(): Promise<void> {
  if (submitting.value) return
  submitting.value = true
  try {
    summary.value = await distributionAPI.apply({ ...applyForm })
    appStore.showSuccess(t('distribution.apply.success'))
    await loadLedger()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.apply.failed')))
  } finally {
    submitting.value = false
  }
}

async function generateBalanceCode(): Promise<void> {
  generating.balance = true
  try {
    const result = await distributionAPI.generateBalanceRedeemCode({ ...balanceForm })
    addGenerated(t('distribution.generated.balanceCode'), result.code, `${formatCurrency(result.cost_rmb, 'CNY')} / $${result.value.toFixed(2)}`)
    await refreshAfterGeneration(result.balance_after)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.generate.failed')))
  } finally {
    generating.balance = false
  }
}

async function generateSubscriptionCode(): Promise<void> {
  if (subscriptionForm.plan_id <= 0) {
    appStore.showError(t('distribution.generate.selectPlanRequired'))
    return
  }
  generating.subscription = true
  try {
    const result = await distributionAPI.generateSubscriptionRedeemCode({ ...subscriptionForm })
    addGenerated(
      t('distribution.generated.subscriptionCode'),
      result.code,
      `${formatCurrency(result.cost_rmb, 'CNY')} / ${result.plan_name || selectedSubscriptionPlan.value?.name || ''} / ${result.validity_days}d`,
    )
    await refreshAfterGeneration(result.balance_after)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.generate.failed')))
  } finally {
    generating.subscription = false
  }
}

async function generateApiKey(): Promise<void> {
  if (apiForm.group_id <= 0) {
    appStore.showError(t('distribution.generate.selectGroupRequired'))
    return
  }
  if (!groups.value.some(group => group.id === apiForm.group_id)) {
    appStore.showError(t('distribution.generate.groupUnavailable'))
    return
  }
  generating.api = true
  try {
    const payload = {
      name: apiForm.name,
      quota_usd: apiForm.quota_usd,
      group_id: apiForm.group_id,
      expires_in_days: apiForm.expires_in_days > 0 ? apiForm.expires_in_days : null,
    }
    const result = await distributionAPI.generateApiKey(payload)
    const copyText = apiKeyCopyText(result.base_url, result.key)
    addGenerated(
      t('distribution.generated.apiKey'),
      copyText,
      `${formatCurrency(result.cost_rmb, 'CNY')} / $${result.quota.toFixed(2)}`,
      copyText,
      true,
    )
    await refreshAfterGeneration(result.balance_after)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.generate.failed')))
  } finally {
    generating.api = false
  }
}

async function refreshAfterGeneration(balanceAfter: number): Promise<void> {
  if (summary.value?.wallet) summary.value.wallet.balance = balanceAfter
  appStore.showSuccess(t('distribution.generate.success'))
  await Promise.all([loadLedger(), loadAssets()])
}

function addGenerated(label: string, value: string, meta: string, copyText?: string, multiline = false): void {
  generatedItems.value.unshift({ label, value, meta, copyText, multiline })
}

async function copy(value: string): Promise<void> {
  await navigator.clipboard.writeText(value)
  appStore.showSuccess(t('common.copiedToClipboard'))
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

function assetCopyText(asset: DistributionAsset): string {
  if (asset.asset_type === 'api_key') {
    const baseUrl = asset.package_url || window.location.origin
    return apiKeyCopyText(baseUrl, asset.display_value)
  }
  return asset.display_value
}

function apiKeyCopyText(baseURL: string, key: string): string {
  const normalizedBaseURL = baseURL || window.location.origin
  const examplePayload = JSON.stringify({
    model: 'gpt-4o-mini',
    messages: [{ role: 'user', content: 'Hello' }],
  })
  return [
    `API Base URL: ${normalizedBaseURL}`,
    `API Key: ${key}`,
    '',
    t('distribution.generated.apiKeyUsageTitle'),
    `1. ${t('distribution.generated.apiKeyUsageBaseUrl', { baseUrl: normalizedBaseURL })}`,
    `2. ${t('distribution.generated.apiKeyUsageBearer')}`,
    `3. ${t('distribution.generated.apiKeyUsageOpenAI')}:`,
    `curl ${normalizedBaseURL}/v1/chat/completions \\`,
    `  -H "Authorization: Bearer ${key}" \\`,
    `  -H "Content-Type: application/json" \\`,
    `  -d '${examplePayload}'`,
  ].join('\n')
}

function formatDiscount(value: number): string {
  return `${(value * 10).toFixed(2).replace(/\.?0+$/, '')}${t('distribution.generate.discountSuffix')}`
}

function canVoidAsset(asset: DistributionAsset): boolean {
  return asset.status === 'active' && !asset.refunded_at
}

async function voidAsset(asset: DistributionAsset): Promise<void> {
  if (!canVoidAsset(asset) || voidingAssetId.value) return
  voidingAssetId.value = asset.id
  try {
    const result = await distributionAPI.voidAsset(asset.id)
    appStore.showSuccess(t('distribution.assets.voidSuccess', { amount: formatCurrency(result.refund_rmb, 'CNY') }))
    if (summary.value?.wallet) summary.value.wallet.balance += result.refund_rmb
    await Promise.all([loadAssets(), loadLedger()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.assets.voidFailed')))
  } finally {
    voidingAssetId.value = null
  }
}

onMounted(async () => {
  try {
    await Promise.all([loadSummary(), loadGroups(), loadSubscriptionPlans()])
    await Promise.all([loadLedger(), loadAssets()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.loadFailed')))
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  if (assetSearchTimer.value) window.clearTimeout(assetSearchTimer.value)
})
</script>
